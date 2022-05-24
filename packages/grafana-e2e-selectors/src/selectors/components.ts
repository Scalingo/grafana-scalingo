// NOTE: by default Component string selectors are set up to be aria-labels,
// however there are many cases where your component may not need an aria-label
// (a <button> with clear text, for example, does not need an aria-label as it's already labeled)
// but you still might need to select it for testing,
// in that case please add the attribute data-test-id={selector} in the component and
// prefix your selector string with 'data-test-id' so that when create the selectors we know to search for it on the right attribute
/**
 * Selectors grouped/defined in Components
 *
 * @alpha
 */
export const Components = {
  TimePicker: {
    openButton: 'data-testid TimePicker Open Button',
    fromField: 'Time Range from field',
    toField: 'Time Range to field',
    applyTimeRange: 'data-testid TimePicker submit button',
    calendar: {
      label: 'Time Range calendar',
      openButton: 'Open time range calendar',
      closeButton: 'Close time range Calendar',
    },
    absoluteTimeRangeTitle: 'data-testid-absolute-time-range-narrow',
  },
  DataSource: {
    TestData: {
      QueryTab: {
        scenarioSelectContainer: 'Test Data Query scenario select container',
        scenarioSelect: 'Test Data Query scenario select',
        max: 'TestData max',
        min: 'TestData min',
        noise: 'TestData noise',
        seriesCount: 'TestData series count',
        spread: 'TestData spread',
        startValue: 'TestData start value',
        drop: 'TestData drop values',
      },
    },
    DataSourceHttpSettings: {
      urlInput: 'Datasource HTTP settings url',
    },
    Jaeger: {
      traceIDInput: 'Trace ID',
    },
    Prometheus: {
      configPage: {
        exemplarsAddButton: 'Add exemplar config button',
        internalLinkSwitch: 'Internal link switch',
      },
      exemplarMarker: 'Exemplar marker',
    },
  },
  Menu: {
    MenuComponent: (title: string) => `${title} menu`,
    MenuGroup: (title: string) => `${title} menu group`,
    MenuItem: (title: string) => `${title} menu item`,
    SubMenu: {
      container: 'SubMenu container',
      icon: 'SubMenu icon',
    },
  },
  Panels: {
    Panel: {
      title: (title: string) => `data-testid Panel header ${title}`,
      headerItems: (item: string) => `Panel header item ${item}`,
      containerByTitle: (title: string) => `${title} panel`,
      headerCornerInfo: (mode: string) => `Panel header ${mode}`,
    },
    Visualization: {
      Graph: {
        VisualizationTab: {
          legendSection: 'Legend section',
        },
        Legend: {
          legendItemAlias: (name: string) => `gpl alias ${name}`,
          showLegendSwitch: 'gpl show legend',
        },
        xAxis: {
          labels: () => 'div.flot-x-axis > div.flot-tick-label',
        },
      },
      BarGauge: {
        /**
         * @deprecated use valueV2 from Grafana 8.3 instead
         */
        value: 'Bar gauge value',
        valueV2: 'data-testid Bar gauge value',
      },
      PieChart: {
        svgSlice: 'Pie Chart Slice',
      },
      Text: {
        container: () => '.markdown-html',
      },
      Table: {
        header: 'table header',
        footer: 'table-footer',
      },
    },
  },
  VizLegend: {
    seriesName: (name: string) => `VizLegend series ${name}`,
  },
  Drawer: {
    General: {
      title: (title: string) => `Drawer title ${title}`,
      expand: 'Drawer expand',
      contract: 'Drawer contract',
      close: 'Drawer close',
      rcContentWrapper: () => '.drawer-content-wrapper',
    },
  },
  PanelEditor: {
    General: {
      content: 'Panel editor content',
    },
    OptionsPane: {
      content: 'Panel editor option pane content',
      select: 'Panel editor option pane select',
      fieldLabel: (type: string) => `${type} field property editor`,
    },
    // not sure about the naming *DataPane*
    DataPane: {
      content: 'Panel editor data pane content',
    },
    applyButton: 'panel editor apply',
    toggleVizPicker: 'toggle-viz-picker',
    toggleVizOptions: 'toggle-viz-options',
    toggleTableView: 'toggle-table-view',
  },
  PanelInspector: {
    Data: {
      content: 'Panel inspector Data content',
    },
    Stats: {
      content: 'Panel inspector Stats content',
    },
    Json: {
      content: 'Panel inspector Json content',
    },
    Query: {
      content: 'Panel inspector Query content',
      refreshButton: 'Panel inspector Query refresh button',
      jsonObjectKeys: () => '.json-formatter-key',
    },
  },
  Tab: {
    title: (title: string) => `Tab ${title}`,
    active: () => '[class*="-activeTabStyle"]',
  },
  RefreshPicker: {
    /**
     * @deprecated use runButtonV2 from Grafana 8.3 instead
     */
    runButton: 'RefreshPicker run button',
    /**
     * @deprecated use intervalButtonV2 from Grafana 8.3 instead
     */
    intervalButton: 'RefreshPicker interval button',
    runButtonV2: 'data-testid RefreshPicker run button',
    intervalButtonV2: 'data-testid RefreshPicker interval button',
  },
  QueryTab: {
    content: 'Query editor tab content',
    queryInspectorButton: 'Query inspector button',
    addQuery: 'Query editor add query button',
  },
  QueryEditorRows: {
    rows: 'Query editor row',
  },
  QueryEditorRow: {
    actionButton: (title: string) => `${title} query operation action`,
    title: (refId: string) => `Query editor row title ${refId}`,
    container: (refId: string) => `Query editor row ${refId}`,
  },
  AlertTab: {
    content: 'Alert editor tab content',
  },
  Alert: {
    /**
     * @deprecated use alertV2 from Grafana 8.3 instead
     */
    alert: (severity: string) => `Alert ${severity}`,
    alertV2: (severity: string) => `data-testid Alert ${severity}`,
  },
  TransformTab: {
    content: 'Transform editor tab content',
    newTransform: (name: string) => `New transform ${name}`,
    transformationEditor: (name: string) => `Transformation editor ${name}`,
    transformationEditorDebugger: (name: string) => `Transformation editor debugger ${name}`,
  },
  Transforms: {
    card: (name: string) => `New transform ${name}`,
    Reduce: {
      modeLabel: 'Transform mode label',
      calculationsLabel: 'Transform calculations label',
    },
    searchInput: 'search transformations',
  },
  PageToolbar: {
    container: () => '.page-toolbar',
    item: (tooltip: string) => `${tooltip}`,
  },
  QueryEditorToolbarItem: {
    button: (title: string) => `QueryEditor toolbar item button ${title}`,
  },
  BackButton: {
    backArrow: 'Go Back',
  },
  OptionsGroup: {
    group: (title?: string) => (title ? `Options group ${title}` : 'Options group'),
    toggle: (title?: string) => (title ? `Options group ${title} toggle` : 'Options group toggle'),
  },
  PluginVisualization: {
    item: (title: string) => `Plugin visualization item ${title}`,
    current: () => '[class*="-currentVisualizationItem"]',
  },
  Select: {
    option: 'Select option',
    input: () => 'input[id*="time-options-input"]',
    singleValue: () => 'div[class*="-singleValue"]',
  },
  FieldConfigEditor: {
    content: 'Field config editor content',
  },
  OverridesConfigEditor: {
    content: 'Field overrides editor content',
  },
  FolderPicker: {
    /**
     * @deprecated use containerV2 from Grafana 8.3 instead
     */
    container: 'Folder picker select container',
    containerV2: 'data-testid Folder picker select container',
    input: 'Select a folder',
  },
  ReadonlyFolderPicker: {
    container: 'data-testid Readonly folder picker select container',
  },
  DataSourcePicker: {
    container: 'Data source picker select container',
    /**
     * @deprecated use inputV2 instead
     */
    input: () => 'input[id="data-source-picker"]',
    inputV2: 'Select a data source',
  },
  TimeZonePicker: {
    /**
     * @deprecated use TimeZonePicker.containerV2 from Grafana 8.3 instead
     */
    container: 'Time zone picker select container',
    containerV2: 'data-testid Time zone picker select container',
  },
  WeekStartPicker: {
    /**
     * @deprecated use WeekStartPicker.containerV2 from Grafana 8.3 instead
     */
    container: 'Choose starting day of the week',
    containerV2: 'data-testid Choose starting day of the week',
    placeholder: 'Choose starting day of the week',
  },
  TraceViewer: {
    spanBar: () => '[data-test-id="SpanBar--wrapper"]',
  },
  QueryField: { container: 'Query field' },
  ValuePicker: {
    button: (name: string) => `Value picker button ${name}`,
    select: (name: string) => `Value picker select ${name}`,
  },
  Search: {
    /**
     * @deprecated use sectionV2 from Grafana 8.3 instead
     */
    section: 'Search section',
    sectionV2: 'data-testid Search section',
    /**
     * @deprecated use itemsV2 from Grafana 8.3 instead
     */
    items: 'Search items',
    itemsV2: 'data-testid Search items',
    cards: 'data-testid Search cards',
    collapseFolder: (sectionId: string) => `data-testid Collapse folder ${sectionId}`,
    expandFolder: (sectionId: string) => `data-testid Expand folder ${sectionId}`,
    dashboardItem: (item: string) => `${Components.Search.dashboardItems} ${item}`,
    dashboardCard: (item: string) => `data-testid Search card ${item}`,
    dashboardItems: 'data-testid Dashboard search item',
  },
  DashboardLinks: {
    container: 'data-testid Dashboard link container',
    dropDown: 'data-testid Dashboard link dropdown',
    link: 'data-testid Dashboard link',
  },
  LoadingIndicator: {
    icon: 'Loading indicator',
  },
  CallToActionCard: {
    /**
     * @deprecated use buttonV2 from Grafana 8.3 instead
     */
    button: (name: string) => `Call to action button ${name}`,
    buttonV2: (name: string) => `data-testid Call to action button ${name}`,
  },
  DataLinksContextMenu: {
    singleLink: 'Data link',
  },
  CodeEditor: {
    container: 'Code editor container',
  },
  DashboardImportPage: {
    textarea: 'data-testid-import-dashboard-textarea',
    submit: 'data-testid-load-dashboard',
  },
  ImportDashboardForm: {
    name: 'data-testid-import-dashboard-title',
    submit: 'data-testid-import-dashboard-submit',
  },
  PanelAlertTabContent: {
    content: 'Unified alert editor tab content',
  },
  VisualizationPreview: {
    card: (name: string) => `data-testid suggestion-${name}`,
  },
  ColorSwatch: {
    name: `data-testid-colorswatch`,
  },
  DashboardRow: {
    title: (title: string) => `data-testid dashboard-row-title-${title}`,
  },
  UserProfile: {
    profileSaveButton: 'data-testid-user-profile-save',
    preferencesSaveButton: 'data-testid-shared-prefs-save',
    orgsTable: 'data-testid-user-orgs-table',
    sessionsTable: 'data-testid-user-sessions-table',
  },
  FileUpload: {
    inputField: 'data-testid-file-upload-input-field',
    fileNameSpan: 'data-testid-file-upload-file-name',
  },
};
