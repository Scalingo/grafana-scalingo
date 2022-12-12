import { cx } from '@emotion/css';
import React, { PureComponent } from 'react';
import { connect, ConnectedProps } from 'react-redux';

import { NavModel, NavModelItem, TimeRange, PageLayoutType, locationUtil } from '@grafana/data';
import { selectors } from '@grafana/e2e-selectors';
import { config, locationService } from '@grafana/runtime';
import { Themeable2, withTheme2 } from '@grafana/ui';
import { notifyApp } from 'app/core/actions';
import { Page } from 'app/core/components/Page/Page';
import { GrafanaContext, GrafanaContextType } from 'app/core/context/GrafanaContext';
import { createErrorNotification } from 'app/core/copy/appNotification';
import { getKioskMode } from 'app/core/navigation/kiosk';
import { GrafanaRouteComponentProps } from 'app/core/navigation/types';
import { getNavModel } from 'app/core/selectors/navModel';
import { PanelModel } from 'app/features/dashboard/state';
import { dashboardWatcher } from 'app/features/live/dashboard/dashboardWatcher';
import { getPageNavFromSlug, getRootContentNavModel } from 'app/features/storage/StorageFolderPage';
import { DashboardRoutes, KioskMode, StoreState } from 'app/types';
import { PanelEditEnteredEvent, PanelEditExitedEvent } from 'app/types/events';

import { cancelVariables, templateVarsChangedInUrl } from '../../variables/state/actions';
import { findTemplateVarChanges } from '../../variables/utils';
import { DashNav } from '../components/DashNav';
import { DashboardFailed } from '../components/DashboardLoading/DashboardFailed';
import { DashboardLoading } from '../components/DashboardLoading/DashboardLoading';
import { DashboardPrompt } from '../components/DashboardPrompt/DashboardPrompt';
import { DashboardSettings } from '../components/DashboardSettings';
import { PanelInspector } from '../components/Inspector/PanelInspector';
import { PanelEditor } from '../components/PanelEditor/PanelEditor';
import { PublicDashboardFooter } from '../components/PublicDashboardFooter/PublicDashboardsFooter';
import { SubMenu } from '../components/SubMenu/SubMenu';
import { DashboardGrid } from '../dashgrid/DashboardGrid';
import { liveTimer } from '../dashgrid/liveTimer';
import { getTimeSrv } from '../services/TimeSrv';
import { cleanUpDashboardAndVariables } from '../state/actions';
import { initDashboard } from '../state/initDashboard';

export interface DashboardPageRouteParams {
  uid?: string;
  type?: string;
  slug?: string;
  accessToken?: string;
}

export type DashboardPageRouteSearchParams = {
  tab?: string;
  folderId?: string;
  editPanel?: string;
  viewPanel?: string;
  editview?: string;
  shareView?: string;
  panelType?: string;
  inspect?: string;
  from?: string;
  to?: string;
  refresh?: string;
  kiosk?: string | true;
};

export const mapStateToProps = (state: StoreState) => ({
  initPhase: state.dashboard.initPhase,
  initError: state.dashboard.initError,
  dashboard: state.dashboard.getModel(),
  navIndex: state.navIndex,
});

const mapDispatchToProps = {
  initDashboard,
  cleanUpDashboardAndVariables,
  notifyApp,
  cancelVariables,
  templateVarsChangedInUrl,
};

const connector = connect(mapStateToProps, mapDispatchToProps);

type OwnProps = {
  isPublic?: boolean;
};

export type Props = OwnProps &
  Themeable2 &
  GrafanaRouteComponentProps<DashboardPageRouteParams, DashboardPageRouteSearchParams> &
  ConnectedProps<typeof connector>;

export interface State {
  editPanel: PanelModel | null;
  viewPanel: PanelModel | null;
  updateScrollTop?: number;
  rememberScrollTop?: number;
  showLoadingState: boolean;
  panelNotFound: boolean;
  editPanelAccessDenied: boolean;
  scrollElement?: HTMLDivElement;
  pageNav?: NavModelItem;
  sectionNav?: NavModel;
}

export class UnthemedDashboardPage extends PureComponent<Props, State> {
  declare context: GrafanaContextType;
  static contextType = GrafanaContext;

  private forceRouteReloadCounter = 0;
  state: State = this.getCleanState();

  getCleanState(): State {
    return {
      editPanel: null,
      viewPanel: null,
      showLoadingState: false,
      panelNotFound: false,
      editPanelAccessDenied: false,
    };
  }

  componentDidMount() {
    this.initDashboard();
    this.forceRouteReloadCounter = (this.props.history.location.state as any)?.routeReloadCounter || 0;
  }

  componentWillUnmount() {
    this.closeDashboard();
  }

  closeDashboard() {
    this.props.cleanUpDashboardAndVariables();
    this.setState(this.getCleanState());
  }

  initDashboard() {
    const { dashboard, isPublic, match, queryParams } = this.props;

    if (dashboard) {
      this.closeDashboard();
    }

    this.props.initDashboard({
      urlSlug: match.params.slug,
      urlUid: match.params.uid,
      urlType: match.params.type,
      urlFolderId: queryParams.folderId,
      panelType: queryParams.panelType,
      routeName: this.props.route.routeName,
      fixUrl: !isPublic,
      accessToken: match.params.accessToken,
      keybindingSrv: this.context.keybindings,
    });

    // small delay to start live updates
    setTimeout(this.updateLiveTimer, 250);
  }

  componentDidUpdate(prevProps: Props, prevState: State) {
    const { dashboard, match, templateVarsChangedInUrl } = this.props;
    const routeReloadCounter = (this.props.history.location.state as any)?.routeReloadCounter;

    if (!dashboard) {
      return;
    }

    if (
      prevProps.match.params.uid !== match.params.uid ||
      (routeReloadCounter !== undefined && this.forceRouteReloadCounter !== routeReloadCounter)
    ) {
      this.initDashboard();
      this.forceRouteReloadCounter = routeReloadCounter;
      return;
    }

    if (prevProps.location.search !== this.props.location.search) {
      const prevUrlParams = prevProps.queryParams;
      const urlParams = this.props.queryParams;

      if (urlParams?.from !== prevUrlParams?.from || urlParams?.to !== prevUrlParams?.to) {
        getTimeSrv().updateTimeRangeFromUrl();
        this.updateLiveTimer();
      }

      if (!prevUrlParams?.refresh && urlParams?.refresh) {
        getTimeSrv().setAutoRefresh(urlParams.refresh);
      }

      const templateVarChanges = findTemplateVarChanges(this.props.queryParams, prevProps.queryParams);

      if (templateVarChanges) {
        templateVarsChangedInUrl(dashboard.uid, templateVarChanges);
      }
    }

    // entering edit mode
    if (this.state.editPanel && !prevState.editPanel) {
      dashboardWatcher.setEditingState(true);

      // Some panels need to be notified when entering edit mode
      this.props.dashboard?.events.publish(new PanelEditEnteredEvent(this.state.editPanel.id));
    }

    // leaving edit mode
    if (!this.state.editPanel && prevState.editPanel) {
      dashboardWatcher.setEditingState(false);

      // Some panels need kicked when leaving edit mode
      this.props.dashboard?.events.publish(new PanelEditExitedEvent(prevState.editPanel.id));
    }

    if (this.state.editPanelAccessDenied) {
      this.props.notifyApp(createErrorNotification('Permission to edit panel denied'));
      locationService.partial({ editPanel: null });
    }

    if (this.state.panelNotFound) {
      this.props.notifyApp(createErrorNotification(`Panel not found`));
      locationService.partial({ editPanel: null, viewPanel: null });
    }
  }

  updateLiveTimer = () => {
    let tr: TimeRange | undefined = undefined;
    if (this.props.dashboard?.liveNow) {
      tr = getTimeSrv().timeRange();
    }
    liveTimer.setLiveTimeRange(tr);
  };

  static getDerivedStateFromProps(props: Props, state: State) {
    const { dashboard, queryParams } = props;

    const urlEditPanelId = queryParams.editPanel;
    const urlViewPanelId = queryParams.viewPanel;

    if (!dashboard) {
      return state;
    }

    const updatedState = { ...state };

    // Entering edit mode
    if (!state.editPanel && urlEditPanelId) {
      const panel = dashboard.getPanelByUrlId(urlEditPanelId);
      if (panel) {
        if (dashboard.canEditPanel(panel)) {
          updatedState.editPanel = panel;
          updatedState.rememberScrollTop = state.scrollElement?.scrollTop;
        } else {
          updatedState.editPanelAccessDenied = true;
        }
      } else {
        updatedState.panelNotFound = true;
      }
    }
    // Leaving edit mode
    else if (state.editPanel && !urlEditPanelId) {
      updatedState.editPanel = null;
      updatedState.updateScrollTop = state.rememberScrollTop;
    }

    // Entering view mode
    if (!state.viewPanel && urlViewPanelId) {
      const panel = dashboard.getPanelByUrlId(urlViewPanelId);
      if (panel) {
        // This mutable state feels wrong to have in getDerivedStateFromProps
        // Should move this state out of dashboard in the future
        dashboard.initViewPanel(panel);
        updatedState.viewPanel = panel;
        updatedState.rememberScrollTop = state.scrollElement?.scrollTop;
        updatedState.updateScrollTop = 0;
      } else {
        updatedState.panelNotFound = true;
      }
    }
    // Leaving view mode
    else if (state.viewPanel && !urlViewPanelId) {
      // This mutable state feels wrong to have in getDerivedStateFromProps
      // Should move this state out of dashboard in the future
      dashboard.exitViewPanel(state.viewPanel);
      updatedState.viewPanel = null;
      updatedState.updateScrollTop = state.rememberScrollTop;
    }

    // if we removed url edit state, clear any panel not found state
    if (state.panelNotFound || (state.editPanelAccessDenied && !urlEditPanelId)) {
      updatedState.panelNotFound = false;
      updatedState.editPanelAccessDenied = false;
    }

    return updateStatePageNavFromProps(props, updatedState);
  }

  onAddPanel = () => {
    const { dashboard } = this.props;

    if (!dashboard) {
      return;
    }

    // Return if the "Add panel" exists already
    if (dashboard.panels.length > 0 && dashboard.panels[0].type === 'add-panel') {
      return;
    }

    // Move all panels down by the height of the "add panel" widget.
    // This is to work around an issue with react-grid-layout that can mess up the layout
    // in certain configurations. (See https://github.com/react-grid-layout/react-grid-layout/issues/1787)
    const addPanelWidgetHeight = 8;
    for (const panel of dashboard.panelIterator()) {
      panel.gridPos.y += addPanelWidgetHeight;
    }

    dashboard.addPanel({
      type: 'add-panel',
      gridPos: { x: 0, y: 0, w: 12, h: addPanelWidgetHeight },
      title: 'Panel Title',
    });

    // scroll to top after adding panel
    this.setState({ updateScrollTop: 0 });
  };

  setScrollRef = (scrollElement: HTMLDivElement): void => {
    this.setState({ scrollElement });
  };

  getInspectPanel() {
    const { dashboard, queryParams } = this.props;

    const inspectPanelId = queryParams.inspect;

    if (!dashboard || !inspectPanelId) {
      return null;
    }

    const inspectPanel = dashboard.getPanelById(parseInt(inspectPanelId, 10));

    // cannot inspect panels plugin is not already loaded
    if (!inspectPanel) {
      return null;
    }

    return inspectPanel;
  }

  render() {
    const { dashboard, initError, queryParams, isPublic } = this.props;
    const { editPanel, viewPanel, updateScrollTop, pageNav, sectionNav } = this.state;
    const kioskMode = !isPublic ? getKioskMode(this.props.queryParams) : KioskMode.Full;

    if (!dashboard || !pageNav || !sectionNav) {
      return <DashboardLoading initPhase={this.props.initPhase} />;
    }

    const inspectPanel = this.getInspectPanel();
    const showSubMenu = !editPanel && !kioskMode && !this.props.queryParams.editview;

    const toolbar = kioskMode !== KioskMode.Full && !queryParams.editview && (
      <header data-testid={selectors.pages.Dashboard.DashNav.navV2}>
        <DashNav
          dashboard={dashboard}
          title={dashboard.title}
          folderTitle={dashboard.meta.folderTitle}
          isFullscreen={!!viewPanel}
          onAddPanel={this.onAddPanel}
          kioskMode={kioskMode}
          hideTimePicker={dashboard.timepicker.hidden}
          shareModalActiveTab={this.props.queryParams.shareView}
        />
      </header>
    );

    const pageClassName = cx({
      'panel-in-fullscreen': Boolean(viewPanel),
      'page-hidden': Boolean(queryParams.editview || editPanel),
    });

    return (
      <>
        <Page
          navModel={sectionNav}
          pageNav={pageNav}
          layout={PageLayoutType.Canvas}
          toolbar={toolbar}
          className={pageClassName}
          scrollRef={this.setScrollRef}
          scrollTop={updateScrollTop}
        >
          <DashboardPrompt dashboard={dashboard} />

          {initError && <DashboardFailed />}
          {showSubMenu && (
            <section aria-label={selectors.pages.Dashboard.SubMenu.submenu}>
              <SubMenu dashboard={dashboard} annotations={dashboard.annotations.list} links={dashboard.links} />
            </section>
          )}

          <DashboardGrid dashboard={dashboard} viewPanel={viewPanel} editPanel={editPanel} />

          {inspectPanel && <PanelInspector dashboard={dashboard} panel={inspectPanel} />}
        </Page>
        {editPanel && (
          <PanelEditor
            dashboard={dashboard}
            sourcePanel={editPanel}
            tab={this.props.queryParams.tab}
            sectionNav={sectionNav}
            pageNav={pageNav}
          />
        )}
        {queryParams.editview && (
          <DashboardSettings
            dashboard={dashboard}
            editview={queryParams.editview}
            pageNav={pageNav}
            sectionNav={sectionNav}
          />
        )}
        {
          // TODO: assess if there are other places where we may want a footer, which may reveal a better place to add this
          isPublic && <PublicDashboardFooter />
        }
      </>
    );
  }
}

function updateStatePageNavFromProps(props: Props, state: State): State {
  const { dashboard } = props;

  if (!dashboard) {
    return state;
  }

  let pageNav = state.pageNav;
  let sectionNav = state.sectionNav;

  if (!pageNav || dashboard.title !== pageNav.text) {
    pageNav = {
      text: dashboard.title,
      url: locationUtil.getUrlForPartial(props.history.location, {
        editview: null,
        editPanel: null,
        viewPanel: null,
      }),
    };
  }

  // Check if folder changed
  const { folderTitle, folderUid } = dashboard.meta;
  if (folderTitle && folderUid && pageNav && pageNav.parentItem?.text !== folderTitle) {
    pageNav = {
      ...pageNav,
      parentItem: {
        text: folderTitle,
        url: `/dashboards/f/${dashboard.meta.folderUid}`,
      },
    };
  }

  if (props.route.routeName === DashboardRoutes.Path) {
    sectionNav = getRootContentNavModel();
    const pageNav = getPageNavFromSlug(props.match.params.slug!);
    if (pageNav?.parentItem) {
      pageNav.parentItem = pageNav.parentItem;
    }
  } else {
    sectionNav = getNavModel(props.navIndex, config.featureToggles.topnav ? 'dashboards/browse' : 'dashboards');
  }

  if (state.editPanel || state.viewPanel) {
    pageNav = {
      ...pageNav,
      text: `${state.editPanel ? 'Edit' : 'View'} panel`,
      parentItem: pageNav,
      url: undefined,
    };
  }

  if (state.pageNav === pageNav && state.sectionNav === sectionNav) {
    return state;
  }

  return {
    ...state,
    pageNav,
    sectionNav,
  };
}

export const DashboardPage = withTheme2(UnthemedDashboardPage);
DashboardPage.displayName = 'DashboardPage';
export default connector(DashboardPage);
