import { css } from '@emotion/css';
import React, { Component } from 'react';
import { ReplaySubject, Subscription } from 'rxjs';

import { GrafanaTheme, PanelProps } from '@grafana/data';
import { config, locationService } from '@grafana/runtime/src';
import { Button, PanelContext, PanelContextRoot, stylesFactory } from '@grafana/ui';
import { CanvasFrameOptions } from 'app/features/canvas';
import { ElementState } from 'app/features/canvas/runtime/element';
import { Scene } from 'app/features/canvas/runtime/scene';
import { PanelEditEnteredEvent, PanelEditExitedEvent } from 'app/types/events';

import { InlineEdit } from './InlineEdit';
import { PanelOptions } from './models.gen';

interface Props extends PanelProps<PanelOptions> {}

interface State {
  refresh: number;
  openInlineEdit: boolean;
}

export interface InstanceState {
  scene: Scene;
  selected: ElementState[];
}

export interface SelectionAction {
  panel: CanvasPanel;
}

let canvasInstances: CanvasPanel[] = [];
let activeCanvasPanel: CanvasPanel | undefined = undefined;
let isInlineEditOpen = false;

export const activePanelSubject = new ReplaySubject<SelectionAction>(1);

export class CanvasPanel extends Component<Props, State> {
  static contextType = PanelContextRoot;
  panelContext: PanelContext = {} as PanelContext;

  readonly scene: Scene;
  private subs = new Subscription();
  needsReload = false;
  styles = getStyles(config.theme);
  isEditing = locationService.getSearchObject().editPanel !== undefined;

  constructor(props: Props) {
    super(props);
    this.state = {
      refresh: 0,
      openInlineEdit: false,
    };

    // Only the initial options are ever used.
    // later changes are all controlled by the scene
    this.scene = new Scene(this.props.options.root, this.props.options.inlineEditing, this.onUpdateScene);
    this.scene.updateSize(props.width, props.height);
    this.scene.updateData(props.data);

    this.subs.add(
      this.props.eventBus.subscribe(PanelEditEnteredEvent, (evt) => {
        // Remove current selection when entering edit mode for any panel in dashboard
        this.scene.clearCurrentSelection();
        this.inlineEditButtonClose();
      })
    );

    this.subs.add(
      this.props.eventBus.subscribe(PanelEditExitedEvent, (evt) => {
        if (this.props.id === evt.payload) {
          this.needsReload = true;
        }
      })
    );
  }

  componentDidMount() {
    activeCanvasPanel = this;
    activePanelSubject.next({ panel: this });

    this.panelContext = this.context as PanelContext;
    if (this.panelContext.onInstanceStateChange) {
      this.panelContext.onInstanceStateChange({
        scene: this.scene,
        layer: this.scene.root,
      });

      this.subs.add(
        this.scene.selection.subscribe({
          next: (v) => {
            this.panelContext.onInstanceStateChange!({
              scene: this.scene,
              selected: v,
              layer: this.scene.root,
            });

            activeCanvasPanel = this;
            activePanelSubject.next({ panel: this });

            canvasInstances.forEach((canvasInstance) => {
              if (canvasInstance !== activeCanvasPanel) {
                canvasInstance.scene.clearCurrentSelection(true);
              }
            });
          },
        })
      );
    }

    canvasInstances.push(this);
  }

  componentWillUnmount() {
    this.subs.unsubscribe();
    isInlineEditOpen = false;
    canvasInstances = canvasInstances.filter((ci) => ci.props.id !== activeCanvasPanel?.props.id);
  }

  // NOTE, all changes to the scene flow through this function
  // even the editor gets current state from the same scene instance!
  onUpdateScene = (root: CanvasFrameOptions) => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      root,
    });

    this.setState({ refresh: this.state.refresh + 1 });
    // console.log('send changes', root);
  };

  shouldComponentUpdate(nextProps: Props, nextState: State) {
    const { width, height, data } = this.props;
    let changed = false;

    if (width !== nextProps.width || height !== nextProps.height) {
      this.scene.updateSize(nextProps.width, nextProps.height);
      changed = true;
    }
    if (data !== nextProps.data) {
      this.scene.updateData(nextProps.data);
      changed = true;
    }

    if (this.state.refresh !== nextState.refresh) {
      changed = true;
    }

    if (this.state.openInlineEdit !== nextState.openInlineEdit) {
      changed = true;
    }

    // After editing, the options are valid, but the scene was in a different panel or inline editing mode has changed
    const shouldUpdateSceneAndPanel = this.needsReload && this.props.options !== nextProps.options;
    const inlineEditingSwitched = this.props.options.inlineEditing !== nextProps.options.inlineEditing;
    if (shouldUpdateSceneAndPanel || inlineEditingSwitched) {
      this.needsReload = false;
      this.scene.load(nextProps.options.root, nextProps.options.inlineEditing);
      this.scene.updateSize(nextProps.width, nextProps.height);
      this.scene.updateData(nextProps.data);
      changed = true;

      if (inlineEditingSwitched && this.props.options.inlineEditing) {
        this.scene.selecto?.destroy();
      }
    }

    return changed;
  }

  inlineEditButtonClick = () => {
    if (isInlineEditOpen) {
      this.forceUpdate();
      this.setActivePanel();
      return;
    }

    this.setActivePanel();
    this.setState({ openInlineEdit: true });
    isInlineEditOpen = true;
  };

  inlineEditButtonClose = () => {
    this.setState({ openInlineEdit: false });
    isInlineEditOpen = false;
  };

  setActivePanel = () => {
    activeCanvasPanel = this;
    activePanelSubject.next({ panel: this });
  };

  renderInlineEdit = () => {
    return <InlineEdit onClose={() => this.inlineEditButtonClose()} />;
  };

  render() {
    return (
      <>
        {this.scene.render()}
        {this.props.options.inlineEditing && !this.isEditing && (
          <div>
            <div className={this.styles.inlineEditButton}>
              <Button
                size="lg"
                variant="secondary"
                icon="edit"
                data-btninlineedit={this.props.id}
                onClick={this.inlineEditButtonClick}
              />
            </div>
            {this.state.openInlineEdit && this.renderInlineEdit()}
          </div>
        )}
      </>
    );
  }
}

const getStyles = stylesFactory((theme: GrafanaTheme) => ({
  inlineEditButton: css`
    position: absolute;
    bottom: 8px;
    left: 8px;
    z-index: 999;
  `,
}));
