// NOTE: This file was auto generated.  DO NOT EDIT DIRECTLY!
// To change feature flags, edit:
//  pkg/services/featuremgmt/registry.go
// Then run tests in:
//  pkg/services/featuremgmt/toggles_gen_test.go

package featuremgmt

const (
	// FlagReturnUnameHeader
	// Return user login as header for authenticated requests
	FlagReturnUnameHeader = "returnUnameHeader"

	// FlagAlertingBigTransactions
	// Use big transactions for alerting database writes
	FlagAlertingBigTransactions = "alertingBigTransactions"

	// FlagPromQueryBuilder
	// Show prometheus query builder
	FlagPromQueryBuilder = "promQueryBuilder"

	// FlagTrimDefaults
	// Use cue schema to remove values that will be applied automatically
	FlagTrimDefaults = "trimDefaults"

	// FlagDisableEnvelopeEncryption
	// Disable envelope encryption (emergency only)
	FlagDisableEnvelopeEncryption = "disableEnvelopeEncryption"

	// FlagDatabaseMetrics
	// Add prometheus metrics for database tables
	FlagDatabaseMetrics = "database_metrics"

	// FlagDashboardPreviews
	// Create and show thumbnails for dashboard search results
	FlagDashboardPreviews = "dashboardPreviews"

	// FlagDashboardPreviewsAdmin
	// Manage the dashboard previews crawler process from the UI
	FlagDashboardPreviewsAdmin = "dashboardPreviewsAdmin"

	// FlagLiveConfig
	// Save grafana live configuration in SQL tables
	FlagLiveConfig = "live-config"

	// FlagLivePipeline
	// enable a generic live processing pipeline
	FlagLivePipeline = "live-pipeline"

	// FlagLiveServiceWebWorker
	// This will use a webworker thread to processes events rather than the main thread
	FlagLiveServiceWebWorker = "live-service-web-worker"

	// FlagQueryOverLive
	// Use grafana live websocket to execute backend queries
	FlagQueryOverLive = "queryOverLive"

	// FlagPanelTitleSearch
	// Search for dashboards using panel title
	FlagPanelTitleSearch = "panelTitleSearch"

	// FlagTempoApmTable
	// Show APM table
	FlagTempoApmTable = "tempoApmTable"

	// FlagPrometheusAzureOverrideAudience
	// Experimental. Allow override default AAD audience for Azure Prometheus endpoint
	FlagPrometheusAzureOverrideAudience = "prometheusAzureOverrideAudience"

	// FlagInfluxdbBackendMigration
	// Query InfluxDB InfluxQL without the proxy
	FlagInfluxdbBackendMigration = "influxdbBackendMigration"

	// FlagShowFeatureFlagsInUI
	// Show feature flags in the settings UI
	FlagShowFeatureFlagsInUI = "showFeatureFlagsInUI"

	// FlagPublicDashboards
	// enables public access to dashboards
	FlagPublicDashboards = "publicDashboards"

	// FlagLokiLive
	// support websocket streaming for loki (early prototype)
	FlagLokiLive = "lokiLive"

	// FlagLokiDataframeApi
	// use experimental loki api for websocket streaming (early prototype)
	FlagLokiDataframeApi = "lokiDataframeApi"

	// FlagLokiMonacoEditor
	// Access to Monaco query editor for Loki
	FlagLokiMonacoEditor = "lokiMonacoEditor"

	// FlagSwaggerUi
	// Serves swagger UI
	FlagSwaggerUi = "swaggerUi"

	// FlagFeatureHighlights
	// Highlight Enterprise features
	FlagFeatureHighlights = "featureHighlights"

	// FlagDashboardComments
	// Enable dashboard-wide comments
	FlagDashboardComments = "dashboardComments"

	// FlagAnnotationComments
	// Enable annotation comments
	FlagAnnotationComments = "annotationComments"

	// FlagMigrationLocking
	// Lock database during migrations
	FlagMigrationLocking = "migrationLocking"

	// FlagStorage
	// Configurable storage for dashboards, datasources, and resources
	FlagStorage = "storage"

	// FlagDashboardsFromStorage
	// Load dashboards from the generic storage interface
	FlagDashboardsFromStorage = "dashboardsFromStorage"

	// FlagExport
	// Export grafana instance (to git, etc)
	FlagExport = "export"

	// FlagAzureMonitorResourcePickerForMetrics
	// New UI for Azure Monitor Metrics Query
	FlagAzureMonitorResourcePickerForMetrics = "azureMonitorResourcePickerForMetrics"

	// FlagExploreMixedDatasource
	// Enable mixed datasource in Explore
	FlagExploreMixedDatasource = "exploreMixedDatasource"

	// FlagTracing
	// Adds trace ID to error notifications
	FlagTracing = "tracing"

	// FlagCommandPalette
	// Enable command palette
	FlagCommandPalette = "commandPalette"

	// FlagCorrelations
	// Correlations page
	FlagCorrelations = "correlations"

	// FlagCloudWatchDynamicLabels
	// Use dynamic labels instead of alias patterns in CloudWatch datasource
	FlagCloudWatchDynamicLabels = "cloudWatchDynamicLabels"

	// FlagDatasourceQueryMultiStatus
	// Introduce HTTP 207 Multi Status for api/ds/query
	FlagDatasourceQueryMultiStatus = "datasourceQueryMultiStatus"

	// FlagTraceToMetrics
	// Enable trace to metrics links
	FlagTraceToMetrics = "traceToMetrics"

	// FlagPrometheusBufferedClient
	// Enable buffered (old) client for Prometheus datasource as default instead of streaming JSON parser client (new)
	FlagPrometheusBufferedClient = "prometheusBufferedClient"

	// FlagNewDBLibrary
	// Use jmoiron/sqlx rather than xorm for a few backend services
	FlagNewDBLibrary = "newDBLibrary"

	// FlagValidateDashboardsOnSave
	// Validate dashboard JSON POSTed to api/dashboards/db
	FlagValidateDashboardsOnSave = "validateDashboardsOnSave"

	// FlagAutoMigrateGraphPanels
	// Replace the angular graph panel with timeseries
	FlagAutoMigrateGraphPanels = "autoMigrateGraphPanels"

	// FlagPrometheusWideSeries
	// Enable wide series responses in the Prometheus datasource
	FlagPrometheusWideSeries = "prometheusWideSeries"

	// FlagCanvasPanelNesting
	// Allow elements nesting
	FlagCanvasPanelNesting = "canvasPanelNesting"

	// FlagScenes
	// Experimental framework to build interactive dashboards
	FlagScenes = "scenes"

	// FlagUseLegacyHeatmapPanel
	// Continue to use the angular/flot based heatmap panel
	FlagUseLegacyHeatmapPanel = "useLegacyHeatmapPanel"

	// FlagDisableSecretsCompatibility
	// Disable duplicated secret storage in legacy tables
	FlagDisableSecretsCompatibility = "disableSecretsCompatibility"

	// FlagLogRequestsInstrumentedAsUnknown
	// Logs the path for requests that are instrumented as unknown
	FlagLogRequestsInstrumentedAsUnknown = "logRequestsInstrumentedAsUnknown"

	// FlagDataConnectionsConsole
	// Enables a new top-level page called Connections. This page is an experiment for better grouping of installing / configuring data sources and other plugins.
	FlagDataConnectionsConsole = "dataConnectionsConsole"

	// FlagInternationalization
	// Enables internationalization
	FlagInternationalization = "internationalization"

	// FlagTopnav
	// New top nav and page layouts
	FlagTopnav = "topnav"

	// FlagGrpcServer
	// Run GRPC server
	FlagGrpcServer = "grpcServer"

	// FlagObjectStore
	// SQL based object store
	FlagObjectStore = "objectStore"

	// FlagTraceqlEditor
	// Show the TraceQL editor in the explore page
	FlagTraceqlEditor = "traceqlEditor"

	// FlagFlameGraph
	// Show the flame graph
	FlagFlameGraph = "flameGraph"

	// FlagCloudWatchCrossAccountQuerying
	// Use cross-account querying in CloudWatch datasource
	FlagCloudWatchCrossAccountQuerying = "cloudWatchCrossAccountQuerying"

	// FlagRedshiftAsyncQueryDataSupport
	// Enable async query data support for Redshift
	FlagRedshiftAsyncQueryDataSupport = "redshiftAsyncQueryDataSupport"

	// FlagAthenaAsyncQueryDataSupport
	// Enable async query data support for Athena
	FlagAthenaAsyncQueryDataSupport = "athenaAsyncQueryDataSupport"

	// FlagIncreaseInMemDatabaseQueryCache
	// Enable more in memory caching for database queries
	FlagIncreaseInMemDatabaseQueryCache = "increaseInMemDatabaseQueryCache"

	// FlagInterFont
	// Switch to inter font
	FlagInterFont = "interFont"

	// FlagNewPanelChromeUI
	// Show updated look and feel of grafana-ui PanelChrome: panel header, icons, and menu
	FlagNewPanelChromeUI = "newPanelChromeUI"

	// FlagQueryLibrary
	// Reusable query library
	FlagQueryLibrary = "queryLibrary"

	// FlagShowDashboardValidationWarnings
	// Show warnings when Dashboards do not validate against the schema
	FlagShowDashboardValidationWarnings = "showDashboardValidationWarnings"

	// FlagMysqlAnsiQuotes
	// Use double quote to escape keyword in Mysql query
	FlagMysqlAnsiQuotes = "mysqlAnsiQuotes"

	// FlagUserRemoteCache
	// Enable using remote cache for users
	FlagUserRemoteCache = "userRemoteCache"

	// FlagDatasourceLogger
	// Logs all datasource requests
	FlagDatasourceLogger = "datasourceLogger"

	// FlagAccessControlOnCall
	// Access control primitives for OnCall
	FlagAccessControlOnCall = "accessControlOnCall"

	// FlagNestedFolders
	// Enable folder nesting
	FlagNestedFolders = "nestedFolders"

	// FlagAccessTokenExpirationCheck
	// Enable OAuth access_token expiration check and token refresh using the refresh_token
	FlagAccessTokenExpirationCheck = "accessTokenExpirationCheck"

	// FlagElasticsearchBackendMigration
	// Use Elasticsearch as backend data source
	FlagElasticsearchBackendMigration = "elasticsearchBackendMigration"

	// FlagSessionRemoteCache
	// Enable using remote cache for user sessions
	FlagSessionRemoteCache = "sessionRemoteCache"
)
