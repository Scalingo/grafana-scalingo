import _ from 'lodash';
import React, { Context } from 'react';

import { Value, Editor as CoreEditor } from 'slate';
import { Editor, Plugin } from '@grafana/slate-react';
import Plain from 'slate-plain-serializer';
import classnames from 'classnames';

import {
  ClearPlugin,
  NewlinePlugin,
  SelectionShortcutsPlugin,
  IndentationPlugin,
  ClipboardPlugin,
  RunnerPlugin,
  SuggestionsPlugin,
} from '../../slate-plugins';

import { makeValue, SCHEMA, CompletionItemGroup, TypeaheadOutput, TypeaheadInput, SuggestionsState } from '../..';

export interface QueryFieldProps {
  additionalPlugins?: Plugin[];
  cleanText?: (text: string) => string;
  disabled?: boolean;
  // We have both value and local state. This is usually an antipattern but we need to keep local state
  // for perf reasons and also have outside value in for example in Explore redux that is mutable from logs
  // creating a two way binding.
  query: string | null;
  onRunQuery?: () => void;
  onBlur?: () => void;
  onChange?: (value: string) => void;
  onTypeahead?: (typeahead: TypeaheadInput) => Promise<TypeaheadOutput>;
  onWillApplySuggestion?: (suggestion: string, state: SuggestionsState) => string;
  placeholder?: string;
  portalOrigin: string;
  syntax?: string;
  syntaxLoaded?: boolean;
}

export interface QueryFieldState {
  suggestions: CompletionItemGroup[];
  typeaheadContext: string | null;
  typeaheadPrefix: string;
  typeaheadText: string;
  value: Value;
}

/**
 * Renders an editor field.
 * Pass initial value as initialQuery and listen to changes in props.onValueChanged.
 * This component can only process strings. Internally it uses Slate Value.
 * Implement props.onTypeahead to use suggestions, see PromQueryField.tsx as an example.
 */
export class QueryField extends React.PureComponent<QueryFieldProps, QueryFieldState> {
  plugins: Plugin[];
  runOnChangeDebounced: Function;
  lastExecutedValue: Value | null = null;
  mounted = false;
  editor: Editor | null = null;

  constructor(props: QueryFieldProps, context: Context<any>) {
    super(props, context);

    this.runOnChangeDebounced = _.debounce(this.runOnChange, 500);

    const { onTypeahead, cleanText, portalOrigin, onWillApplySuggestion } = props;

    // Base plugins
    this.plugins = [
      NewlinePlugin(),
      SuggestionsPlugin({ onTypeahead, cleanText, portalOrigin, onWillApplySuggestion }),
      ClearPlugin(),
      RunnerPlugin({ handler: this.runOnChangeAndRunQuery }),
      SelectionShortcutsPlugin(),
      IndentationPlugin(),
      ClipboardPlugin(),
      ...(props.additionalPlugins || []),
    ].filter(p => p);

    this.state = {
      suggestions: [],
      typeaheadContext: null,
      typeaheadPrefix: '',
      typeaheadText: '',
      value: makeValue(props.query || '', props.syntax),
    };
  }

  componentDidMount() {
    this.mounted = true;
  }

  componentWillUnmount() {
    this.mounted = false;
  }

  componentDidUpdate(prevProps: QueryFieldProps, prevState: QueryFieldState) {
    const { query, syntax } = this.props;
    const { value } = this.state;

    // Handle two way binging between local state and outside prop.
    // if query changed from the outside
    if (query !== prevProps.query) {
      // and we have a version that differs
      if (query !== Plain.serialize(value)) {
        this.setState({ value: makeValue(query || '', syntax) });
      }
    }
  }

  UNSAFE_componentWillReceiveProps(nextProps: QueryFieldProps) {
    if (nextProps.syntaxLoaded && !this.props.syntaxLoaded) {
      if (!this.editor) {
        return;
      }

      // Need a bogus edit to re-render the editor after syntax has fully loaded
      const editor = this.editor.insertText(' ').deleteBackward(1);
      this.onChange(editor.value, true);
    }
  }

  /**
   * Update local state, propagate change upstream and optionally run the query afterwards.
   */
  onChange = (value: Value, runQuery?: boolean) => {
    const documentChanged = value.document !== this.state.value.document;
    const prevValue = this.state.value;

    // Update local state with new value and optionally change value upstream.
    this.setState({ value }, () => {
      // The diff is needed because the actual value of editor have much more metadata (for example text selection)
      // that is not passed upstream so every change of editor value does not mean change of the query text.
      if (documentChanged) {
        const textChanged = Plain.serialize(prevValue) !== Plain.serialize(value);
        if (textChanged && runQuery) {
          this.runOnChangeAndRunQuery();
        }
        if (textChanged && !runQuery) {
          // Debounce change propagation by default for perf reasons.
          this.runOnChangeDebounced();
        }
      }
    });
  };

  runOnChange = () => {
    const { onChange } = this.props;

    if (onChange) {
      onChange(Plain.serialize(this.state.value));
    }
  };

  runOnRunQuery = () => {
    const { onRunQuery } = this.props;

    if (onRunQuery) {
      onRunQuery();
      this.lastExecutedValue = this.state.value;
    }
  };

  runOnChangeAndRunQuery = () => {
    // onRunQuery executes query from Redux in Explore so it needs to be updated sync in case we want to run
    // the query.
    this.runOnChange();
    this.runOnRunQuery();
  };

  /**
   * We need to handle blur events here mainly because of dashboard panels which expect to have query executed on blur.
   */
  handleBlur = (event: Event, editor: CoreEditor, next: Function) => {
    const { onBlur } = this.props;
    if (onBlur) {
      onBlur();
    } else {
      // Run query by default on blur
      const previousValue = this.lastExecutedValue ? Plain.serialize(this.lastExecutedValue) : null;
      const currentValue = Plain.serialize(editor.value);

      if (previousValue !== currentValue) {
        this.runOnChangeAndRunQuery();
      }
    }
    return next();
  };

  render() {
    const { disabled } = this.props;
    const wrapperClassName = classnames('slate-query-field__wrapper', {
      'slate-query-field__wrapper--disabled': disabled,
    });

    return (
      <div className={wrapperClassName}>
        <div className="slate-query-field">
          <Editor
            ref={editor => (this.editor = editor!)}
            schema={SCHEMA}
            autoCorrect={false}
            readOnly={this.props.disabled}
            onBlur={this.handleBlur}
            // onKeyDown={this.onKeyDown}
            onChange={(change: { value: Value }) => {
              this.onChange(change.value, false);
            }}
            placeholder={this.props.placeholder}
            plugins={this.plugins}
            spellCheck={false}
            value={this.state.value}
          />
        </div>
      </div>
    );
  }
}

export default QueryField;
