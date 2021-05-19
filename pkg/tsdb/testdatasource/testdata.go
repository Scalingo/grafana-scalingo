package testdatasource

import (
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/plugins/backendplugin/coreplugin"
	"github.com/grafana/grafana/pkg/registry"
)

func init() {
	registry.RegisterService(&testDataPlugin{})
}

type testDataPlugin struct {
	BackendPluginManager backendplugin.Manager `inject:""`
	logger               log.Logger
	scenarios            map[string]*Scenario
	queryMux             *datasource.QueryTypeMux
}

func (p *testDataPlugin) Init() error {
	p.logger = log.New("tsdb.testdata")
	p.scenarios = map[string]*Scenario{}
	p.queryMux = datasource.NewQueryTypeMux()
	p.registerScenarios()
	resourceMux := http.NewServeMux()
	p.registerRoutes(resourceMux)
	factory := coreplugin.New(backend.ServeOpts{
		QueryDataHandler:    p.queryMux,
		CallResourceHandler: httpadapter.New(resourceMux),
	})
	err := p.BackendPluginManager.Register("testdata", factory)
	if err != nil {
		p.logger.Error("Failed to register plugin", "error", err)
	}
	return nil
}
