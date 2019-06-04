import { ComponentClass } from 'react';
import { TimeRange } from './time';
import { PluginMeta, GrafanaPlugin } from './plugin';
import { TableData, TimeSeries, SeriesData, LoadingState } from './data';
import { PanelData } from './panel';

// NOTE: this seems more general than just DataSource
export interface DataSourcePluginOptionsEditorProps<TOptions> {
  options: TOptions;
  onOptionsChange: (options: TOptions) => void;
}

export class DataSourcePlugin<
  TOptions extends DataSourceJsonData = DataSourceJsonData,
  TQuery extends DataQuery = DataQuery
> extends GrafanaPlugin<DataSourcePluginMeta> {
  DataSourceClass: DataSourceConstructor<TQuery>;
  components: DataSourcePluginComponents<TOptions, TQuery>;

  constructor(DataSourceClass: DataSourceConstructor<TQuery>) {
    super();
    this.DataSourceClass = DataSourceClass;
    this.components = {};
  }

  setConfigEditor(editor: React.ComponentType<DataSourcePluginOptionsEditorProps<DataSourceSettings<TOptions>>>) {
    this.components.ConfigEditor = editor;
    return this;
  }

  setQueryCtrl(QueryCtrl: any) {
    this.components.QueryCtrl = QueryCtrl;
    return this;
  }

  setAnnotationQueryCtrl(AnnotationsQueryCtrl: any) {
    this.components.AnnotationsQueryCtrl = AnnotationsQueryCtrl;
    return this;
  }

  setQueryEditor(QueryEditor: ComponentClass<QueryEditorProps<DataSourceApi, TQuery>>) {
    this.components.QueryEditor = QueryEditor;
    return this;
  }

  setExploreQueryField(ExploreQueryField: ComponentClass<ExploreQueryFieldProps<DataSourceApi, TQuery>>) {
    this.components.ExploreQueryField = ExploreQueryField;
    return this;
  }

  setExploreStartPage(ExploreStartPage: ComponentClass<ExploreStartPageProps>) {
    this.components.ExploreStartPage = ExploreStartPage;
    return this;
  }

  setVariableQueryEditor(VariableQueryEditor: any) {
    this.components.VariableQueryEditor = VariableQueryEditor;
    return this;
  }

  setComponentsFromLegacyExports(pluginExports: any) {
    this.angularConfigCtrl = pluginExports.ConfigCtrl;

    this.components.QueryCtrl = pluginExports.QueryCtrl;
    this.components.AnnotationsQueryCtrl = pluginExports.AnnotationsQueryCtrl;
    this.components.ExploreQueryField = pluginExports.ExploreQueryField;
    this.components.ExploreStartPage = pluginExports.ExploreStartPage;
    this.components.QueryEditor = pluginExports.QueryEditor;
    this.components.VariableQueryEditor = pluginExports.VariableQueryEditor;
  }
}

export interface DataSourcePluginMeta extends PluginMeta {
  builtIn?: boolean; // Is this for all
  metrics?: boolean;
  tables?: boolean;
  logs?: boolean;
  explore?: boolean;
  annotations?: boolean;
  mixed?: boolean;
  hasQueryHelp?: boolean;
  queryOptions?: PluginMetaQueryOptions;
  sort?: number;
  /**
   * By default, hidden queries are not passed to the datasource
   * Set this to true in plugin.json to have hidden queries passed to the
   * DataSource query method
   */
  hiddenQueries?: boolean;
}

interface PluginMetaQueryOptions {
  cacheTimeout?: boolean;
  maxDataPoints?: boolean;
  minInterval?: boolean;
}

export interface DataSourcePluginComponents<
  TOptions extends DataSourceJsonData = DataSourceJsonData,
  TQuery extends DataQuery = DataQuery
> {
  QueryCtrl?: any;
  AnnotationsQueryCtrl?: any;
  VariableQueryEditor?: any;
  QueryEditor?: ComponentClass<QueryEditorProps<DataSourceApi, TQuery>>;
  ExploreQueryField?: ComponentClass<ExploreQueryFieldProps<DataSourceApi, TQuery>>;
  ExploreStartPage?: ComponentClass<ExploreStartPageProps>;
  ConfigEditor?: React.ComponentType<DataSourcePluginOptionsEditorProps<DataSourceSettings<TOptions>>>;
}

export interface DataSourceConstructor<TQuery extends DataQuery = DataQuery> {
  new (instanceSettings: DataSourceInstanceSettings, ...args: any[]): DataSourceApi<TQuery>;
}

/**
 * The main data source abstraction interface, represents an instance of a data source
 */
export interface DataSourceApi<TQuery extends DataQuery = DataQuery> {
  /**
   *  min interval range
   */
  interval?: string;

  /**
   * Imports queries from a different datasource
   */
  importQueries?(queries: TQuery[], originMeta: PluginMeta): Promise<TQuery[]>;

  /**
   * Initializes a datasource after instantiation
   */
  init?: () => void;

  /**
   * Main metrics / data query action
   */
  query(options: DataQueryRequest<TQuery>, observer?: DataStreamObserver): Promise<DataQueryResponse>;

  /**
   * Test & verify datasource settings & connection details
   */
  testDatasource(): Promise<any>;

  /**
   *  Get hints for query improvements
   */
  getQueryHints?(query: TQuery, results: any[], ...rest: any): QueryHint[];

  /**
   * Convert a query to a simple text string
   */
  getQueryDisplayText?(query: TQuery): string;

  /**
   *  Set after constructor is called by Grafana
   */
  name?: string;

  /**
   *  Set after constructor is called by Grafana
   */
  id?: number;

  /**
   * Set after constructor call, as the data source instance is the most common thing to pass around
   * we attach the components to this instance for easy access
   */
  components?: DataSourcePluginComponents;

  /**
   * static information about the datasource
   */
  meta?: DataSourcePluginMeta;
}

export interface ExploreDataSourceApi<TQuery extends DataQuery = DataQuery> extends DataSourceApi {
  modifyQuery?(query: TQuery, action: QueryFixAction): TQuery;
  getHighlighterExpression?(query: TQuery): string;
  languageProvider?: any;
}

export interface QueryEditorProps<DSType extends DataSourceApi, TQuery extends DataQuery> {
  datasource: DSType;
  query: TQuery;
  onRunQuery: () => void;
  onChange: (value: TQuery) => void;
  panelData: PanelData; // The current panel data
  queryResponse?: PanelData; // data filtered to only this query.  Includes the error.
}

export enum DataSourceStatus {
  Connected,
  Disconnected,
}

export interface ExploreQueryFieldProps<DSType extends DataSourceApi, TQuery extends DataQuery> {
  datasource: DSType;
  datasourceStatus: DataSourceStatus;
  query: TQuery;
  error?: string | JSX.Element;
  hint?: QueryHint;
  history: any[];
  onExecuteQuery?: () => void;
  onQueryChange?: (value: TQuery) => void;
  onExecuteHint?: (action: QueryFixAction) => void;
}

export interface ExploreStartPageProps {
  onClickExample: (query: DataQuery) => void;
}

/**
 * Starting in v6.2 SeriesData can represent both TimeSeries and TableData
 */
export type LegacyResponseData = TimeSeries | TableData | any;

export type DataQueryResponseData = SeriesData | LegacyResponseData;

export type DataStreamObserver = (event: DataStreamState) => void;

export interface DataStreamState {
  /**
   * when Done or Error no more events will be processed
   */
  state: LoadingState;

  /**
   * Consistent key across events.
   */
  key: string;

  /**
   * The stream request.  The properties of this request will be examined
   * to determine if the stream matches the original query.  If not, it
   * will be unsubscribed.
   */
  request: DataQueryRequest;

  /**
   * Series data may not be known yet
   */
  series?: SeriesData[];

  /**
   * Error in stream (but may still be running)
   */
  error?: DataQueryError;

  /**
   * Optionally return only the rows that changed in this event
   */
  delta?: SeriesData[];

  /**
   * Stop listening to this stream
   */
  unsubscribe: () => void;
}

export interface DataQueryResponse {
  data: DataQueryResponseData[];
}

export interface DataQuery {
  /**
   * A - Z
   */
  refId: string;

  /**
   * true if query is disabled (ie not executed / sent to TSDB)
   */
  hide?: boolean;

  /**
   * Unique, guid like, string used in explore mode
   */
  key?: string;

  /**
   * For mixed data sources the selected datasource is on the query level.
   * For non mixed scenarios this is undefined.
   */
  datasource?: string | null;
}

export interface DataQueryError {
  data?: {
    message?: string;
    error?: string;
  };
  message?: string;
  status?: string;
  statusText?: string;
  refId?: string;
}

export interface ScopedVar {
  text: any;
  value: any;
  [key: string]: any;
}

export interface ScopedVars {
  [key: string]: ScopedVar;
}

export interface DataQueryRequest<TQuery extends DataQuery = DataQuery> {
  requestId: string; // Used to identify results and optionally cancel the request in backendSrv
  timezone: string;
  range: TimeRange;
  timeInfo?: string; // The query time description (blue text in the upper right)
  targets: TQuery[];
  panelId: number;
  dashboardId: number;
  cacheTimeout?: string;
  interval: string;
  intervalMs: number;
  maxDataPoints: number;
  scopedVars: ScopedVars;

  // Request Timing
  startTime: number;
  endTime?: number;
}

export interface QueryFix {
  type: string;
  label: string;
  action?: QueryFixAction;
}

export interface QueryFixAction {
  type: string;
  query?: string;
  preventSubmit?: boolean;
}

export interface QueryHint {
  type: string;
  label: string;
  fix?: QueryFix;
}

export interface DataSourceJsonData {
  authType?: string;
  defaultRegion?: string;
}

/**
 * Data Source instance edit model.  This is returned from:
 *  /api/datasources
 */
export interface DataSourceSettings<T extends DataSourceJsonData = DataSourceJsonData, S = {}> {
  id: number;
  orgId: number;
  name: string;
  typeLogoUrl: string;
  type: string;
  access: string;
  url: string;
  password: string;
  user: string;
  database: string;
  basicAuth: boolean;
  basicAuthPassword: string;
  basicAuthUser: string;
  isDefault: boolean;
  jsonData: T;
  secureJsonData?: S;
  readOnly: boolean;
  withCredentials: boolean;
}

/**
 * Frontend settings model that is passed to Datasource constructor. This differs a bit from the model above
 * as this data model is available to every user who has access to a data source (Viewers+).  This is loaded
 * in bootData (on page load), or from: /api/frontend/settings
 */
export interface DataSourceInstanceSettings<T extends DataSourceJsonData = DataSourceJsonData> {
  id: number;
  type: string;
  name: string;
  meta: DataSourcePluginMeta;
  url?: string;
  jsonData: T;
  username?: string;
  password?: string; // when access is direct, for some legacy datasources

  /**
   * This is the full Authorization header if basic auth is ennabled.
   * Only available here when access is Browser (direct), when acess is Server (proxy)
   * The basic auth header, username & password is never exposted to browser/Frontend
   * so this will be emtpy then.
   */
  basicAuth?: string;
  withCredentials?: boolean;
}

export interface DataSourceSelectItem {
  name: string;
  value: string | null;
  meta: DataSourcePluginMeta;
  sort: string;
}
