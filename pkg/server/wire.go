//go:build wireinject
// +build wireinject

package server

import (
	"github.com/google/wire"
	sdkhttpclient "github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana/pkg/api"
	"github.com/grafana/grafana/pkg/api/avatar"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/cuectx"
	"github.com/grafana/grafana/pkg/expr"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/httpclient"
	"github.com/grafana/grafana/pkg/infra/httpclient/httpclientprovider"
	"github.com/grafana/grafana/pkg/infra/kvstore"
	"github.com/grafana/grafana/pkg/infra/localcache"
	"github.com/grafana/grafana/pkg/infra/metrics"
	"github.com/grafana/grafana/pkg/infra/remotecache"
	"github.com/grafana/grafana/pkg/infra/serverlock"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	uss "github.com/grafana/grafana/pkg/infra/usagestats/service"
	"github.com/grafana/grafana/pkg/infra/usagestats/statscollector"
	loginpkg "github.com/grafana/grafana/pkg/login"
	"github.com/grafana/grafana/pkg/login/social"
	"github.com/grafana/grafana/pkg/middleware/csrf"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/backendplugin/coreplugin"
	pluginsCfg "github.com/grafana/grafana/pkg/plugins/config"
	"github.com/grafana/grafana/pkg/plugins/manager"
	"github.com/grafana/grafana/pkg/plugins/manager/client"
	pluginDashboards "github.com/grafana/grafana/pkg/plugins/manager/dashboards"
	"github.com/grafana/grafana/pkg/plugins/manager/loader"
	processManager "github.com/grafana/grafana/pkg/plugins/manager/process"
	"github.com/grafana/grafana/pkg/plugins/manager/registry"
	managerStore "github.com/grafana/grafana/pkg/plugins/manager/store"
	"github.com/grafana/grafana/pkg/plugins/plugincontext"
	"github.com/grafana/grafana/pkg/plugins/repo"
	"github.com/grafana/grafana/pkg/registry/corekind"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/acimpl"
	"github.com/grafana/grafana/pkg/services/accesscontrol/ossaccesscontrol"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/annotations"
	"github.com/grafana/grafana/pkg/services/annotations/annotationsimpl"
	"github.com/grafana/grafana/pkg/services/apikey/apikeyimpl"
	"github.com/grafana/grafana/pkg/services/auth"
	"github.com/grafana/grafana/pkg/services/auth/jwt"
	"github.com/grafana/grafana/pkg/services/cleanup"
	"github.com/grafana/grafana/pkg/services/comments"
	"github.com/grafana/grafana/pkg/services/contexthandler"
	"github.com/grafana/grafana/pkg/services/contexthandler/authproxy"
	"github.com/grafana/grafana/pkg/services/correlations"
	"github.com/grafana/grafana/pkg/services/dashboardimport"
	dashboardimportservice "github.com/grafana/grafana/pkg/services/dashboardimport/service"
	"github.com/grafana/grafana/pkg/services/dashboards"
	dashboardstore "github.com/grafana/grafana/pkg/services/dashboards/database"
	dashboardservice "github.com/grafana/grafana/pkg/services/dashboards/service"
	"github.com/grafana/grafana/pkg/services/dashboardsnapshots"
	dashsnapstore "github.com/grafana/grafana/pkg/services/dashboardsnapshots/database"
	dashsnapsvc "github.com/grafana/grafana/pkg/services/dashboardsnapshots/service"
	"github.com/grafana/grafana/pkg/services/dashboardversion/dashverimpl"
	"github.com/grafana/grafana/pkg/services/datasourceproxy"
	"github.com/grafana/grafana/pkg/services/datasources"
	datasourceservice "github.com/grafana/grafana/pkg/services/datasources/service"
	"github.com/grafana/grafana/pkg/services/encryption"
	encryptionservice "github.com/grafana/grafana/pkg/services/encryption/service"
	"github.com/grafana/grafana/pkg/services/export"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/folder/folderimpl"
	"github.com/grafana/grafana/pkg/services/grpcserver"
	grpccontext "github.com/grafana/grafana/pkg/services/grpcserver/context"
	"github.com/grafana/grafana/pkg/services/grpcserver/interceptors"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/hooks"
	"github.com/grafana/grafana/pkg/services/libraryelements"
	"github.com/grafana/grafana/pkg/services/librarypanels"
	"github.com/grafana/grafana/pkg/services/live"
	"github.com/grafana/grafana/pkg/services/live/pushhttp"
	"github.com/grafana/grafana/pkg/services/login"
	"github.com/grafana/grafana/pkg/services/login/authinfoservice"
	authinfodatabase "github.com/grafana/grafana/pkg/services/login/authinfoservice/database"
	"github.com/grafana/grafana/pkg/services/login/loginservice"
	"github.com/grafana/grafana/pkg/services/loginattempt/loginattemptimpl"
	"github.com/grafana/grafana/pkg/services/navtree/navtreeimpl"
	"github.com/grafana/grafana/pkg/services/ngalert"
	ngimage "github.com/grafana/grafana/pkg/services/ngalert/image"
	ngmetrics "github.com/grafana/grafana/pkg/services/ngalert/metrics"
	ngstore "github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/services/notifications"
	"github.com/grafana/grafana/pkg/services/oauthtoken"
	"github.com/grafana/grafana/pkg/services/org/orgimpl"
	"github.com/grafana/grafana/pkg/services/playlist/playlistimpl"
	"github.com/grafana/grafana/pkg/services/plugindashboards"
	plugindashboardsservice "github.com/grafana/grafana/pkg/services/plugindashboards/service"
	"github.com/grafana/grafana/pkg/services/pluginsettings"
	pluginSettings "github.com/grafana/grafana/pkg/services/pluginsettings/service"
	"github.com/grafana/grafana/pkg/services/preference/prefimpl"
	"github.com/grafana/grafana/pkg/services/publicdashboards"
	publicdashboardsApi "github.com/grafana/grafana/pkg/services/publicdashboards/api"
	publicdashboardsStore "github.com/grafana/grafana/pkg/services/publicdashboards/database"
	publicdashboardsService "github.com/grafana/grafana/pkg/services/publicdashboards/service"
	"github.com/grafana/grafana/pkg/services/query"
	"github.com/grafana/grafana/pkg/services/queryhistory"
	"github.com/grafana/grafana/pkg/services/querylibrary/querylibraryimpl"
	"github.com/grafana/grafana/pkg/services/quota/quotaimpl"
	"github.com/grafana/grafana/pkg/services/rendering"
	"github.com/grafana/grafana/pkg/services/search"
	"github.com/grafana/grafana/pkg/services/searchV2"
	"github.com/grafana/grafana/pkg/services/secrets"
	secretsDatabase "github.com/grafana/grafana/pkg/services/secrets/database"
	secretsStore "github.com/grafana/grafana/pkg/services/secrets/kvstore"
	secretsMigrations "github.com/grafana/grafana/pkg/services/secrets/kvstore/migrations"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
	secretsMigrator "github.com/grafana/grafana/pkg/services/secrets/migrator"
	"github.com/grafana/grafana/pkg/services/serviceaccounts"
	"github.com/grafana/grafana/pkg/services/serviceaccounts/database"
	serviceaccountsmanager "github.com/grafana/grafana/pkg/services/serviceaccounts/manager"
	"github.com/grafana/grafana/pkg/services/shorturls"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/sqlstore/mockstore"
	"github.com/grafana/grafana/pkg/services/star/starimpl"
	"github.com/grafana/grafana/pkg/services/store"
	"github.com/grafana/grafana/pkg/services/store/kind"
	"github.com/grafana/grafana/pkg/services/store/object/httpobjectstore"
	"github.com/grafana/grafana/pkg/services/store/object/sqlstash"
	"github.com/grafana/grafana/pkg/services/store/resolver"
	"github.com/grafana/grafana/pkg/services/store/sanitizer"
	"github.com/grafana/grafana/pkg/services/tag"
	"github.com/grafana/grafana/pkg/services/tag/tagimpl"
	"github.com/grafana/grafana/pkg/services/team/teamimpl"
	"github.com/grafana/grafana/pkg/services/teamguardian"
	teamguardianDatabase "github.com/grafana/grafana/pkg/services/teamguardian/database"
	teamguardianManager "github.com/grafana/grafana/pkg/services/teamguardian/manager"
	tempuser "github.com/grafana/grafana/pkg/services/temp_user"
	"github.com/grafana/grafana/pkg/services/temp_user/tempuserimpl"
	"github.com/grafana/grafana/pkg/services/thumbs"
	"github.com/grafana/grafana/pkg/services/thumbs/dashboardthumbsimpl"
	"github.com/grafana/grafana/pkg/services/updatechecker"
	"github.com/grafana/grafana/pkg/services/user/userimpl"
	"github.com/grafana/grafana/pkg/services/userauth/userauthimpl"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/azuremonitor"
	"github.com/grafana/grafana/pkg/tsdb/cloudmonitoring"
	"github.com/grafana/grafana/pkg/tsdb/cloudwatch"
	"github.com/grafana/grafana/pkg/tsdb/elasticsearch"
	"github.com/grafana/grafana/pkg/tsdb/grafanads"
	"github.com/grafana/grafana/pkg/tsdb/graphite"
	"github.com/grafana/grafana/pkg/tsdb/influxdb"
	"github.com/grafana/grafana/pkg/tsdb/legacydata"
	legacydataservice "github.com/grafana/grafana/pkg/tsdb/legacydata/service"
	"github.com/grafana/grafana/pkg/tsdb/loki"
	"github.com/grafana/grafana/pkg/tsdb/mssql"
	"github.com/grafana/grafana/pkg/tsdb/mysql"
	"github.com/grafana/grafana/pkg/tsdb/opentsdb"
	"github.com/grafana/grafana/pkg/tsdb/parca"
	"github.com/grafana/grafana/pkg/tsdb/phlare"
	"github.com/grafana/grafana/pkg/tsdb/postgres"
	"github.com/grafana/grafana/pkg/tsdb/prometheus"
	"github.com/grafana/grafana/pkg/tsdb/tempo"
	"github.com/grafana/grafana/pkg/tsdb/testdatasource"
)

var wireBasicSet = wire.NewSet(
	legacydataservice.ProvideService,
	wire.Bind(new(legacydata.RequestHandler), new(*legacydataservice.Service)),
	annotationsimpl.ProvideService,
	wire.Bind(new(annotations.Repository), new(*annotationsimpl.RepositoryImpl)),
	alerting.ProvideAlertStore,
	alerting.ProvideAlertEngine,
	wire.Bind(new(alerting.UsageStatsQuerier), new(*alerting.AlertEngine)),
	setting.NewCfgFromArgs,
	New,
	api.ProvideHTTPServer,
	query.ProvideService,
	bus.ProvideBus,
	wire.Bind(new(bus.Bus), new(*bus.InProcBus)),
	thumbs.ProvideService,
	rendering.ProvideService,
	wire.Bind(new(rendering.Service), new(*rendering.RenderingService)),
	routing.ProvideRegister,
	wire.Bind(new(routing.RouteRegister), new(*routing.RouteRegisterImpl)),
	hooks.ProvideService,
	kvstore.ProvideService,
	localcache.ProvideService,
	dashboardthumbsimpl.ProvideService,
	updatechecker.ProvideGrafanaService,
	updatechecker.ProvidePluginsService,
	uss.ProvideService,
	wire.Bind(new(usagestats.Service), new(*uss.UsageStats)),
	registry.ProvideService,
	wire.Bind(new(registry.Service), new(*registry.InMemory)),
	pluginsCfg.ProvideConfig,
	repo.ProvideService,
	wire.Bind(new(repo.Service), new(*repo.Manager)),
	manager.ProvideInstaller,
	wire.Bind(new(plugins.Installer), new(*manager.PluginInstaller)),
	client.ProvideService,
	wire.Bind(new(plugins.Client), new(*client.Service)),
	managerStore.ProvideService,
	wire.Bind(new(plugins.Store), new(*managerStore.Service)),
	wire.Bind(new(plugins.RendererManager), new(*managerStore.Service)),
	wire.Bind(new(plugins.SecretsPluginManager), new(*managerStore.Service)),
	wire.Bind(new(plugins.StaticRouteResolver), new(*managerStore.Service)),
	pluginDashboards.ProvideFileStoreManager,
	wire.Bind(new(pluginDashboards.FileStore), new(*pluginDashboards.FileStoreManager)),
	processManager.ProvideService,
	wire.Bind(new(processManager.Service), new(*processManager.Manager)),
	coreplugin.ProvideCoreRegistry,
	loader.ProvideService,
	wire.Bind(new(loader.Service), new(*loader.Loader)),
	wire.Bind(new(plugins.ErrorResolver), new(*loader.Loader)),
	cloudwatch.ProvideService,
	cloudmonitoring.ProvideService,
	azuremonitor.ProvideService,
	postgres.ProvideService,
	mysql.ProvideService,
	mssql.ProvideService,
	store.ProvideEntityEventsService,
	httpclientprovider.New,
	wire.Bind(new(httpclient.Provider), new(*sdkhttpclient.Provider)),
	serverlock.ProvideService,
	annotationsimpl.ProvideCleanupService,
	wire.Bind(new(annotations.Cleaner), new(*annotationsimpl.CleanupServiceImpl)),
	cleanup.ProvideService,
	shorturls.ProvideService,
	wire.Bind(new(shorturls.Service), new(*shorturls.ShortURLService)),
	queryhistory.ProvideService,
	wire.Bind(new(queryhistory.Service), new(*queryhistory.QueryHistoryService)),
	correlations.ProvideService,
	wire.Bind(new(correlations.Service), new(*correlations.CorrelationsService)),
	quotaimpl.ProvideService,
	remotecache.ProvideService,
	loginservice.ProvideService,
	wire.Bind(new(login.Service), new(*loginservice.Implementation)),
	authinfoservice.ProvideAuthInfoService,
	wire.Bind(new(login.AuthInfoService), new(*authinfoservice.Implementation)),
	authinfodatabase.ProvideAuthInfoStore,
	loginpkg.ProvideService,
	wire.Bind(new(loginpkg.Authenticator), new(*loginpkg.AuthenticatorService)),
	datasourceproxy.ProvideService,
	search.ProvideService,
	searchV2.ProvideService,
	searchV2.ProvideSearchHTTPService,
	store.ProvideService,
	export.ProvideService,
	live.ProvideService,
	pushhttp.ProvideService,
	plugincontext.ProvideService,
	contexthandler.ProvideService,
	jwt.ProvideService,
	wire.Bind(new(models.JWTService), new(*jwt.AuthService)),
	ngstore.ProvideDBStore,
	ngimage.ProvideDeleteExpiredService,
	ngalert.ProvideService,
	librarypanels.ProvideService,
	wire.Bind(new(librarypanels.Service), new(*librarypanels.LibraryPanelService)),
	libraryelements.ProvideService,
	wire.Bind(new(libraryelements.Service), new(*libraryelements.LibraryElementService)),
	notifications.ProvideService,
	notifications.ProvideSmtpService,
	tracing.ProvideService,
	metrics.ProvideService,
	testdatasource.ProvideService,
	opentsdb.ProvideService,
	social.ProvideService,
	influxdb.ProvideService,
	wire.Bind(new(social.Service), new(*social.SocialService)),
	oauthtoken.ProvideService,
	auth.ProvideActiveAuthTokenService,
	wire.Bind(new(auth.ActiveTokenService), new(*auth.ActiveAuthTokenService)),
	wire.Bind(new(oauthtoken.OAuthTokenService), new(*oauthtoken.Service)),
	tempo.ProvideService,
	loki.ProvideService,
	graphite.ProvideService,
	prometheus.ProvideService,
	elasticsearch.ProvideService,
	phlare.ProvideService,
	parca.ProvideService,
	encryptionservice.ProvideEncryptionService,
	wire.Bind(new(encryption.Internal), new(*encryptionservice.Service)),
	secretsManager.ProvideSecretsService,
	wire.Bind(new(secrets.Service), new(*secretsManager.SecretsService)),
	secretsDatabase.ProvideSecretsStore,
	querylibraryimpl.ProvideService,
	querylibraryimpl.ProvideHTTPService,
	wire.Bind(new(secrets.Store), new(*secretsDatabase.SecretsStoreImpl)),
	secretsMigrator.ProvideSecretsMigrator,
	wire.Bind(new(secrets.Migrator), new(*secretsMigrator.SecretsMigrator)),
	grafanads.ProvideService,
	wire.Bind(new(dashboardsnapshots.Store), new(*dashsnapstore.DashboardSnapshotStore)),
	dashsnapstore.ProvideStore,
	wire.Bind(new(dashboardsnapshots.Service), new(*dashsnapsvc.ServiceImpl)),
	dashsnapsvc.ProvideService,
	datasourceservice.ProvideService,
	wire.Bind(new(datasources.DataSourceService), new(*datasourceservice.Service)),
	pluginSettings.ProvideService,
	wire.Bind(new(pluginsettings.Service), new(*pluginSettings.Service)),
	alerting.ProvideService,
	database.ProvideServiceAccountsStore,
	wire.Bind(new(serviceaccounts.Store), new(*database.ServiceAccountsStoreImpl)),
	ossaccesscontrol.ProvideServiceAccountPermissions,
	wire.Bind(new(accesscontrol.ServiceAccountPermissionsService), new(*ossaccesscontrol.ServiceAccountPermissionsService)),
	serviceaccountsmanager.ProvideServiceAccountsService,
	wire.Bind(new(serviceaccounts.Service), new(*serviceaccountsmanager.ServiceAccountsService)),
	expr.ProvideService,
	teamguardianDatabase.ProvideTeamGuardianStore,
	wire.Bind(new(teamguardian.Store), new(*teamguardianDatabase.TeamGuardianStoreImpl)),
	teamguardianManager.ProvideService,
	featuremgmt.ProvideManagerService,
	featuremgmt.ProvideToggles,
	dashboardservice.ProvideDashboardService,
	dashboardstore.ProvideDashboardStore,
	folderimpl.ProvideService,
	wire.Bind(new(dashboards.DashboardService), new(*dashboardservice.DashboardServiceImpl)),
	wire.Bind(new(dashboards.DashboardProvisioningService), new(*dashboardservice.DashboardServiceImpl)),
	wire.Bind(new(dashboards.PluginService), new(*dashboardservice.DashboardServiceImpl)),
	wire.Bind(new(dashboards.Store), new(*dashboardstore.DashboardStore)),
	dashboardimportservice.ProvideService,
	wire.Bind(new(dashboardimport.Service), new(*dashboardimportservice.ImportDashboardService)),
	plugindashboardsservice.ProvideService,
	wire.Bind(new(plugindashboards.Service), new(*plugindashboardsservice.Service)),
	plugindashboardsservice.ProvideDashboardUpdater,
	alerting.ProvideDashAlertExtractorService,
	wire.Bind(new(alerting.DashAlertExtractor), new(*alerting.DashAlertExtractorService)),
	comments.ProvideService,
	guardian.ProvideService,
	sanitizer.ProvideService,
	secretsStore.ProvideService,
	avatar.ProvideAvatarCacheServer,
	authproxy.ProvideAuthProxy,
	statscollector.ProvideService,
	corekind.KindSet,
	cuectx.GrafanaCUEContext,
	cuectx.GrafanaThemaRuntime,
	csrf.ProvideCSRFFilter,
	ossaccesscontrol.ProvideTeamPermissions,
	wire.Bind(new(accesscontrol.TeamPermissionsService), new(*ossaccesscontrol.TeamPermissionsService)),
	ossaccesscontrol.ProvideFolderPermissions,
	wire.Bind(new(accesscontrol.FolderPermissionsService), new(*ossaccesscontrol.FolderPermissionsService)),
	ossaccesscontrol.ProvideDashboardPermissions,
	wire.Bind(new(accesscontrol.DashboardPermissionsService), new(*ossaccesscontrol.DashboardPermissionsService)),
	starimpl.ProvideService,
	playlistimpl.ProvideService,
	apikeyimpl.ProvideService,
	dashverimpl.ProvideService,
	publicdashboardsService.ProvideService,
	wire.Bind(new(publicdashboards.Service), new(*publicdashboardsService.PublicDashboardServiceImpl)),
	publicdashboardsStore.ProvideStore,
	wire.Bind(new(publicdashboards.Store), new(*publicdashboardsStore.PublicDashboardStoreImpl)),
	publicdashboardsApi.ProvideApi,
	userimpl.ProvideService,
	orgimpl.ProvideService,
	grpccontext.ProvideContextHandler,
	grpcserver.ProvideService,
	grpcserver.ProvideHealthService,
	grpcserver.ProvideReflectionService,
	interceptors.ProvideAuthenticator,
	kind.ProvideService, // The registry of known kinds
	sqlstash.ProvideSQLObjectServer,
	resolver.ProvideObjectReferenceResolver,
	httpobjectstore.ProvideHTTPObjectStore,
	teamimpl.ProvideService,
	tempuserimpl.ProvideService,
	loginattemptimpl.ProvideService,
	userauthimpl.ProvideService,
	secretsMigrations.ProvideDataSourceMigrationService,
	secretsMigrations.ProvideMigrateToPluginService,
	secretsMigrations.ProvideMigrateFromPluginService,
	secretsMigrations.ProvideSecretMigrationProvider,
	wire.Bind(new(secretsMigrations.SecretMigrationProvider), new(*secretsMigrations.SecretMigrationProviderImpl)),
	acimpl.ProvideAccessControl,
	navtreeimpl.ProvideService,
	wire.Bind(new(accesscontrol.AccessControl), new(*acimpl.AccessControl)),
	wire.Bind(new(notifications.TempUserStore), new(tempuser.Service)),
	tagimpl.ProvideService,
	wire.Bind(new(tag.Service), new(*tagimpl.Service)),
)

var wireSet = wire.NewSet(
	wireBasicSet,
	sqlstore.ProvideService,
	ngmetrics.ProvideService,
	wire.Bind(new(notifications.Service), new(*notifications.NotificationService)),
	wire.Bind(new(notifications.WebhookSender), new(*notifications.NotificationService)),
	wire.Bind(new(notifications.EmailSender), new(*notifications.NotificationService)),
	wire.Bind(new(sqlstore.Store), new(*sqlstore.SQLStore)),
	wire.Bind(new(db.DB), new(*sqlstore.SQLStore)),
	prefimpl.ProvideService,
)

var wireTestSet = wire.NewSet(
	wireBasicSet,
	ProvideTestEnv,
	sqlstore.ProvideServiceForTests,
	ngmetrics.ProvideServiceForTest,

	notifications.MockNotificationService,
	wire.Bind(new(notifications.Service), new(*notifications.NotificationServiceMock)),
	wire.Bind(new(notifications.WebhookSender), new(*notifications.NotificationServiceMock)),
	wire.Bind(new(notifications.EmailSender), new(*notifications.NotificationServiceMock)),
	mockstore.NewSQLStoreMock,
	wire.Bind(new(sqlstore.Store), new(*sqlstore.SQLStore)),
	wire.Bind(new(db.DB), new(*sqlstore.SQLStore)),
	prefimpl.ProvideService,
)

func Initialize(cla setting.CommandLineArgs, opts Options, apiOpts api.ServerOptions) (*Server, error) {
	wire.Build(wireExtsSet)
	return &Server{}, nil
}

func InitializeForTest(cla setting.CommandLineArgs, opts Options, apiOpts api.ServerOptions) (*TestEnv, error) {
	wire.Build(wireExtsTestSet)
	return &TestEnv{Server: &Server{}, SQLStore: &sqlstore.SQLStore{}}, nil
}
