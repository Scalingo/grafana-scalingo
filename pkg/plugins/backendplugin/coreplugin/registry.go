package coreplugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/tsdb/azuremonitor"
	"github.com/grafana/grafana/pkg/tsdb/cloudmonitoring"
	"github.com/grafana/grafana/pkg/tsdb/cloudwatch"
	"github.com/grafana/grafana/pkg/tsdb/elasticsearch"
	"github.com/grafana/grafana/pkg/tsdb/grafanads"
	"github.com/grafana/grafana/pkg/tsdb/graphite"
	"github.com/grafana/grafana/pkg/tsdb/influxdb"
	"github.com/grafana/grafana/pkg/tsdb/loki"
	"github.com/grafana/grafana/pkg/tsdb/mssql"
	"github.com/grafana/grafana/pkg/tsdb/mysql"
	"github.com/grafana/grafana/pkg/tsdb/opentsdb"
	"github.com/grafana/grafana/pkg/tsdb/postgres"
	"github.com/grafana/grafana/pkg/tsdb/prometheus"
	"github.com/grafana/grafana/pkg/tsdb/tempo"
	"github.com/grafana/grafana/pkg/tsdb/testdatasource"
)

const (
	CloudWatch      = "cloudwatch"
	CloudMonitoring = "stackdriver"
	AzureMonitor    = "grafana-azure-monitor-datasource"
	Elasticsearch   = "elasticsearch"
	Graphite        = "graphite"
	InfluxDB        = "influxdb"
	Loki            = "loki"
	OpenTSDB        = "opentsdb"
	Prometheus      = "prometheus"
	Tempo           = "tempo"
	TestData        = "testdata"
	PostgreSQL      = "postgres"
	MySQL           = "mysql"
	MSSQL           = "mssql"
	Grafana         = "grafana"
)

type Registry struct {
	store map[string]backendplugin.PluginFactoryFunc
}

func NewRegistry(store map[string]backendplugin.PluginFactoryFunc) *Registry {
	return &Registry{
		store: store,
	}
}

func ProvideCoreRegistry(am *azuremonitor.Service, cw *cloudwatch.CloudWatchService, cm *cloudmonitoring.Service,
	es *elasticsearch.Service, grap *graphite.Service, idb *influxdb.Service, lk *loki.Service, otsdb *opentsdb.Service,
	pr *prometheus.Service, t *tempo.Service, td *testdatasource.Service, pg *postgres.Service, my *mysql.Service,
	ms *mssql.Service, graf *grafanads.Service) *Registry {
	return NewRegistry(map[string]backendplugin.PluginFactoryFunc{
		CloudWatch:      asBackendPlugin(cw.Executor),
		CloudMonitoring: asBackendPlugin(cm),
		AzureMonitor:    asBackendPlugin(am),
		Elasticsearch:   asBackendPlugin(es),
		Graphite:        asBackendPlugin(grap),
		InfluxDB:        asBackendPlugin(idb),
		Loki:            asBackendPlugin(lk),
		OpenTSDB:        asBackendPlugin(otsdb),
		Prometheus:      asBackendPlugin(pr),
		Tempo:           asBackendPlugin(t),
		TestData:        asBackendPlugin(td),
		PostgreSQL:      asBackendPlugin(pg),
		MySQL:           asBackendPlugin(my),
		MSSQL:           asBackendPlugin(ms),
		Grafana:         asBackendPlugin(graf),
	})
}

func (cr *Registry) Get(pluginID string) backendplugin.PluginFactoryFunc {
	return cr.store[pluginID]
}

func (cr *Registry) BackendFactoryProvider() func(_ context.Context, p *plugins.Plugin) backendplugin.PluginFactoryFunc {
	return func(_ context.Context, p *plugins.Plugin) backendplugin.PluginFactoryFunc {
		if !p.IsCorePlugin() {
			return nil
		}

		return cr.Get(p.ID)
	}
}

func asBackendPlugin(svc interface{}) backendplugin.PluginFactoryFunc {
	opts := backend.ServeOpts{}
	if queryHandler, ok := svc.(backend.QueryDataHandler); ok {
		opts.QueryDataHandler = queryHandler
	}
	if resourceHandler, ok := svc.(backend.CallResourceHandler); ok {
		opts.CallResourceHandler = resourceHandler
	}
	if streamHandler, ok := svc.(backend.StreamHandler); ok {
		opts.StreamHandler = streamHandler
	}
	if healthHandler, ok := svc.(backend.CheckHealthHandler); ok {
		opts.CheckHealthHandler = healthHandler
	}

	if opts.QueryDataHandler != nil || opts.CallResourceHandler != nil ||
		opts.CheckHealthHandler != nil || opts.StreamHandler != nil {
		return New(opts)
	}

	return nil
}
