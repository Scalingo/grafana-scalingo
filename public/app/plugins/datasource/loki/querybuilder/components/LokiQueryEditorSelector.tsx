import React, { SyntheticEvent, useCallback, useEffect, useState } from 'react';

import { CoreApp, LoadingState } from '@grafana/data';
import { EditorHeader, EditorRows, FlexItem, InlineSelect, Space } from '@grafana/experimental';
import { reportInteraction } from '@grafana/runtime';
import { Button, ConfirmModal } from '@grafana/ui';
import { FeedbackLink } from 'app/plugins/datasource/prometheus/querybuilder/shared/FeedbackLink';
import { QueryEditorModeToggle } from 'app/plugins/datasource/prometheus/querybuilder/shared/QueryEditorModeToggle';
import { QueryHeaderSwitch } from 'app/plugins/datasource/prometheus/querybuilder/shared/QueryHeaderSwitch';
import { QueryEditorMode } from 'app/plugins/datasource/prometheus/querybuilder/shared/types';

import { LokiQueryEditorProps } from '../../components/types';
import { LokiQuery } from '../../types';
import { lokiQueryModeller } from '../LokiQueryModeller';
import { buildVisualQueryFromString } from '../parsing';
import { changeEditorMode, getQueryWithDefaults, useRawQuery } from '../state';

import { LokiQueryBuilderContainer } from './LokiQueryBuilderContainer';
import { LokiQueryBuilderExplained } from './LokiQueryBuilderExplained';
import { LokiQueryBuilderOptions } from './LokiQueryBuilderOptions';
import { LokiQueryCodeEditor } from './LokiQueryCodeEditor';

export const LokiQueryEditorSelector = React.memo<LokiQueryEditorProps>((props) => {
  const { onChange, onRunQuery, data, app } = props;
  const [parseModalOpen, setParseModalOpen] = useState(false);
  const [dataIsStale, setDataIsStale] = useState(false);

  const query = getQueryWithDefaults(props.query);
  const [rawQuery, setRawQuery] = useRawQuery();
  // This should be filled in from the defaults by now.
  const editorMode = query.editorMode!;

  const onEditorModeChange = useCallback(
    (newEditorMode: QueryEditorMode) => {
      reportInteraction('grafana_loki_editor_mode_clicked', {
        newEditor: newEditorMode,
        previousEditor: query.editorMode ?? '',
        newQuery: !query.expr,
        app: app ?? '',
      });

      if (newEditorMode === QueryEditorMode.Builder) {
        const result = buildVisualQueryFromString(query.expr || '');
        // If there are errors, give user a chance to decide if they want to go to builder as that can loose some data.
        if (result.errors.length) {
          setParseModalOpen(true);
          return;
        }
      }
      changeEditorMode(query, newEditorMode, onChange);
    },
    [onChange, query, app]
  );

  useEffect(() => {
    setDataIsStale(false);
  }, [data]);

  const onChangeInternal = (query: LokiQuery) => {
    setDataIsStale(true);
    onChange(query);
  };

  const onQueryPreviewChange = (event: SyntheticEvent<HTMLInputElement>) => {
    const isEnabled = event.currentTarget.checked;
    setRawQuery(isEnabled);
  };

  return (
    <>
      <ConfirmModal
        isOpen={parseModalOpen}
        title="Query parsing"
        body="There were errors while trying to parse the query. Continuing to visual builder may loose some parts of the query."
        confirmText="Continue"
        onConfirm={() => {
          onChange({ ...query, editorMode: QueryEditorMode.Builder });
          setParseModalOpen(false);
        }}
        onDismiss={() => setParseModalOpen(false)}
      />
      <EditorHeader>
        <InlineSelect
          value={null}
          placeholder="Query patterns"
          allowCustomValue
          onChange={({ value }) => {
            const result = buildVisualQueryFromString(query.expr || '');
            result.query.operations = value?.operations!;
            onChange({
              ...query,
              expr: lokiQueryModeller.renderQuery(result.query),
            });
          }}
          options={lokiQueryModeller.getQueryPatterns().map((x) => ({ label: x.name, value: x }))}
        />
        {editorMode === QueryEditorMode.Builder && (
          <>
            <QueryHeaderSwitch label="Raw query" value={rawQuery} onChange={onQueryPreviewChange} />
            <FeedbackLink feedbackUrl="https://github.com/grafana/grafana/discussions/50785" />
          </>
        )}
        <FlexItem grow={1} />
        {app !== CoreApp.Explore && (
          <Button
            variant={dataIsStale ? 'primary' : 'secondary'}
            size="sm"
            onClick={onRunQuery}
            icon={data?.state === LoadingState.Loading ? 'fa fa-spinner' : undefined}
            disabled={data?.state === LoadingState.Loading}
          >
            Run queries
          </Button>
        )}
        <QueryEditorModeToggle mode={editorMode!} onChange={onEditorModeChange} />
      </EditorHeader>
      <Space v={0.5} />
      <EditorRows>
        {editorMode === QueryEditorMode.Code && <LokiQueryCodeEditor {...props} onChange={onChangeInternal} />}
        {editorMode === QueryEditorMode.Builder && (
          <LokiQueryBuilderContainer
            datasource={props.datasource}
            query={query}
            onChange={onChangeInternal}
            onRunQuery={props.onRunQuery}
            showRawQuery={rawQuery}
          />
        )}
        {editorMode === QueryEditorMode.Explain && <LokiQueryBuilderExplained query={query.expr} />}
        {editorMode !== QueryEditorMode.Explain && (
          <LokiQueryBuilderOptions query={query} onChange={onChange} onRunQuery={onRunQuery} app={app} />
        )}
      </EditorRows>
    </>
  );
});

LokiQueryEditorSelector.displayName = 'LokiQueryEditorSelector';
