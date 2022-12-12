package navtreeimpl

import (
	"net/http"
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
	accesscontrolmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/navtree"
	"github.com/grafana/grafana/pkg/services/pluginsettings"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
	"github.com/stretchr/testify/require"
)

func TestAddAppLinks(t *testing.T) {
	httpReq, _ := http.NewRequest(http.MethodGet, "", nil)
	reqCtx := &models.ReqContext{SignedInUser: &user.SignedInUser{}, Context: &web.Context{Req: httpReq}}
	permissions := []ac.Permission{
		{Action: plugins.ActionAppAccess, Scope: "*"},
	}

	testApp1 := plugins.PluginDTO{
		JSONData: plugins.JSONData{
			ID:   "test-app1",
			Name: "Test app1 name",
			Type: plugins.App,
			Includes: []*plugins.Includes{
				{
					Name:       "Catalog",
					Path:       "/a/test-app1/catalog",
					Type:       "page",
					AddToNav:   true,
					DefaultNav: true,
				},
				{
					Name:     "Page2",
					Path:     "/a/test-app1/page2",
					Type:     "page",
					AddToNav: true,
				},
			},
		},
	}

	testApp2 := plugins.PluginDTO{
		JSONData: plugins.JSONData{
			ID:   "test-app2",
			Name: "Test app2 name",
			Type: plugins.App,
			Includes: []*plugins.Includes{
				{
					Name:       "Hello",
					Path:       "/a/quick-app/catalog",
					Type:       "page",
					AddToNav:   true,
					DefaultNav: true,
				},
			},
		},
	}

	testApp3 := plugins.PluginDTO{
		JSONData: plugins.JSONData{
			ID:   "test-app3",
			Name: "Test app3 name",
			Type: plugins.App,
			Includes: []*plugins.Includes{
				{
					Name:       "Default page",
					Path:       "/a/test-app3/default",
					Type:       "page",
					AddToNav:   true,
					DefaultNav: true,
				},
				{
					Name:     "Random page",
					Path:     "/a/test-app3/random-page",
					Type:     "page",
					AddToNav: true,
				},
				{
					Name:     "Connect data",
					Path:     "/connections/connect-data",
					Type:     "page",
					AddToNav: false,
				},
			},
		},
	}

	pluginSettings := pluginsettings.FakePluginSettings{Plugins: map[string]*pluginsettings.DTO{
		testApp1.ID: {ID: 0, OrgID: 1, PluginID: testApp1.ID, PluginVersion: "1.0.0", Enabled: true},
		testApp2.ID: {ID: 0, OrgID: 1, PluginID: testApp2.ID, PluginVersion: "1.0.0", Enabled: true},
		testApp3.ID: {ID: 0, OrgID: 1, PluginID: testApp3.ID, PluginVersion: "1.0.0", Enabled: true},
	}}

	service := ServiceImpl{
		log:            log.New("navtree"),
		cfg:            setting.NewCfg(),
		accessControl:  accesscontrolmock.New().WithPermissions(permissions),
		pluginSettings: &pluginSettings,
		features:       featuremgmt.WithFeatures(),
		pluginStore: plugins.FakePluginStore{
			PluginList: []plugins.PluginDTO{testApp1, testApp2, testApp3},
		},
	}

	t.Run("Should add enabled apps with pages", func(t *testing.T) {
		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		require.Equal(t, "Test app1 name", treeRoot.Children[0].Text)
		require.Equal(t, "/a/test-app1/catalog", treeRoot.Children[0].Url)
		require.Equal(t, "/a/test-app1/page2", treeRoot.Children[0].Children[1].Url)
	})

	t.Run("Should move apps to Apps category when topnav is enabled", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)

		appsNode := treeRoot.FindById(navtree.NavIDApps)
		require.NotNil(t, appsNode)
		require.Equal(t, "Apps", appsNode.Text)
		require.Len(t, appsNode.Children, 3)
		require.Equal(t, testApp1.Name, appsNode.Children[0].Text)
	})

	t.Run("Should remove the default nav child (DefaultNav=true) when topnav is enabled and should set its URL to the plugin nav root", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)

		app1Node := treeRoot.FindById("plugin-page-test-app1")
		require.Len(t, app1Node.Children, 1) // The page include with DefaultNav=true gets removed
		require.Equal(t, "/a/test-app1/catalog", app1Node.Url)
		require.Equal(t, "Page2", app1Node.Children[0].Text)
	})

	// This can be done by using `[navigation.app_sections]` in the INI config
	t.Run("Should move apps that have root nav id configured to the root", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app1": {SectionID: navtree.NavIDRoot},
		}

		treeRoot := navtree.NavTreeRoot{}

		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)

		// Check if the plugin gets moved to the root
		require.Len(t, treeRoot.Children, 2)
		require.Equal(t, "plugin-page-test-app1", treeRoot.Children[0].Id)

		// Check if it is not under the "Apps" section anymore
		appsNode := treeRoot.FindById(navtree.NavIDApps)
		require.NotNil(t, appsNode)
		require.Len(t, appsNode.Children, 2)
		require.Equal(t, "plugin-page-test-app2", appsNode.Children[0].Id)
		require.Equal(t, "plugin-page-test-app3", appsNode.Children[1].Id)
	})

	// This can be done by using `[navigation.app_sections]` in the INI config
	t.Run("Should move apps that have specific nav id configured to correct section", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app1": {SectionID: navtree.NavIDAdmin},
		}

		treeRoot := navtree.NavTreeRoot{}
		treeRoot.AddSection(&navtree.NavLink{
			Id: navtree.NavIDAdmin,
		})

		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)

		// Check if the plugin gets moved over to the "Admin" section
		adminNode := treeRoot.FindById(navtree.NavIDAdmin)
		require.NotNil(t, adminNode)
		require.Len(t, adminNode.Children, 1)
		require.Equal(t, "plugin-page-test-app1", adminNode.Children[0].Id)

		// Check if it is not under the "Apps" section anymore
		appsNode := treeRoot.FindById(navtree.NavIDApps)
		require.NotNil(t, appsNode)
		require.Len(t, appsNode.Children, 2)
		require.Equal(t, "plugin-page-test-app2", appsNode.Children[0].Id)
		require.Equal(t, "plugin-page-test-app3", appsNode.Children[1].Id)
	})

	t.Run("Should only add a 'Monitoring' section if a plugin exists that wants to live there", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		service.navigationAppConfig = map[string]NavigationAppConfig{}

		// Check if the Monitoring section is not there if no apps try to register to it
		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		monitoringNode := treeRoot.FindById(navtree.NavIDMonitoring)
		require.Nil(t, monitoringNode)

		// It should appear and once an app tries to register to it
		treeRoot = navtree.NavTreeRoot{}
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app1": {SectionID: navtree.NavIDMonitoring},
		}
		err = service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		monitoringNode = treeRoot.FindById(navtree.NavIDMonitoring)
		require.NotNil(t, monitoringNode)
		require.Len(t, monitoringNode.Children, 1)
		require.Equal(t, "Test app1 name", monitoringNode.Children[0].Text)
	})

	t.Run("Should add a 'Alerts and Incidents' section if a plugin exists that wants to live there", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		service.navigationAppConfig = map[string]NavigationAppConfig{}

		// Check if the 'Alerts and Incidents' section is not there if no apps try to register to it
		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		alertsAndIncidentsNode := treeRoot.FindById(navtree.NavIDAlertsAndIncidents)
		require.Nil(t, alertsAndIncidentsNode)

		// If there is no 'Alerting' node in the navigation (= alerting not enabled) then we don't auto-create the 'Alerts and Incidents' section
		treeRoot = navtree.NavTreeRoot{}
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app1": {SectionID: navtree.NavIDAlertsAndIncidents},
		}
		err = service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		alertsAndIncidentsNode = treeRoot.FindById(navtree.NavIDAlertsAndIncidents)
		require.Nil(t, alertsAndIncidentsNode)

		// It should appear and once an app tries to register to it and the `Alerting` nav node is present
		treeRoot = navtree.NavTreeRoot{}
		treeRoot.AddSection(&navtree.NavLink{Id: navtree.NavIDAlerting, Text: "Alerting"})
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app1": {SectionID: navtree.NavIDAlertsAndIncidents},
		}
		err = service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)
		alertsAndIncidentsNode = treeRoot.FindById(navtree.NavIDAlertsAndIncidents)
		require.NotNil(t, alertsAndIncidentsNode)
		require.Len(t, alertsAndIncidentsNode.Children, 2)
		require.Equal(t, "Alerting", alertsAndIncidentsNode.Children[0].Text)
		require.Equal(t, "Test app1 name", alertsAndIncidentsNode.Children[1].Text)
	})

	t.Run("Should be able to control app sort order with SortWeight (smaller SortWeight displayed first)", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav)
		service.navigationAppConfig = map[string]NavigationAppConfig{
			"test-app2": {SectionID: navtree.NavIDMonitoring, SortWeight: 2},
			"test-app1": {SectionID: navtree.NavIDMonitoring, SortWeight: 3},
			"test-app3": {SectionID: navtree.NavIDMonitoring, SortWeight: 1},
		}

		treeRoot := navtree.NavTreeRoot{}
		err := service.addAppLinks(&treeRoot, reqCtx)
		treeRoot.Sort()
		monitoringNode := treeRoot.FindById(navtree.NavIDMonitoring)

		require.NoError(t, err)
		require.Equal(t, "Test app3 name", monitoringNode.Children[0].Text)
		require.Equal(t, "Test app2 name", monitoringNode.Children[1].Text)
		require.Equal(t, "Test app1 name", monitoringNode.Children[2].Text)
	})

	t.Run("Should replace page from plugin", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav, featuremgmt.FlagDataConnectionsConsole)
		service.navigationAppConfig = map[string]NavigationAppConfig{}
		service.navigationAppPathConfig = map[string]NavigationAppConfig{
			"/connections/connect-data": {SectionID: "connections"},
		}

		treeRoot := navtree.NavTreeRoot{}
		treeRoot.AddSection(service.buildDataConnectionsNavLink(reqCtx))
		connectionsNode := treeRoot.FindById("connections")
		require.Equal(t, "Connections", connectionsNode.Text)
		require.Equal(t, "Connect data", connectionsNode.Children[1].Text)
		require.Equal(t, "connections-connect-data", connectionsNode.Children[1].Id) // Original "Connect data" page
		require.Equal(t, "", connectionsNode.Children[1].PluginID)

		err := service.addAppLinks(&treeRoot, reqCtx)

		// Check if the standalone plugin page appears under the section where we registered it
		require.NoError(t, err)
		require.Equal(t, "Connections", connectionsNode.Text)
		require.Equal(t, "Connect data", connectionsNode.Children[1].Text)
		require.Equal(t, "standalone-plugin-page-/connections/connect-data", connectionsNode.Children[1].Id) // Overridden "Connect data" page
		require.Equal(t, "test-app3", connectionsNode.Children[1].PluginID)

		// Check if the standalone plugin page does not appear under the app section anymore
		// (Also checking if the Default Page got removed)
		app3Node := treeRoot.FindById("plugin-page-test-app3")
		require.NotNil(t, app3Node)
		require.Len(t, app3Node.Children, 1)
		require.Equal(t, "Random page", app3Node.Children[0].Text)

		// The plugin item should take the URL of the Default Nav
		require.Equal(t, "/a/test-app3/default", app3Node.Url)
	})

	t.Run("Should not register pages under the app plugin section unless AddToNav=true", func(t *testing.T) {
		service.features = featuremgmt.WithFeatures(featuremgmt.FlagTopnav, featuremgmt.FlagDataConnectionsConsole)
		service.navigationAppPathConfig = map[string]NavigationAppConfig{} // We don't configure it as a standalone plugin page

		treeRoot := navtree.NavTreeRoot{}
		treeRoot.AddSection(service.buildDataConnectionsNavLink(reqCtx))
		err := service.addAppLinks(&treeRoot, reqCtx)
		require.NoError(t, err)

		// The original core page should exist under the section
		connectDataNode := treeRoot.FindById("connections-connect-data")
		require.Equal(t, "connections-connect-data", connectDataNode.Id)
		require.Equal(t, "", connectDataNode.PluginID)

		// The standalone plugin page should not be found in the navtree at all (as we didn't configure it)
		standaloneConnectDataNode := treeRoot.FindById("standalone-plugin-page-/connections/connect-data")
		require.Nil(t, standaloneConnectDataNode)

		// Only the pages that have `AddToNav=true` appear under the plugin navigation
		app3Node := treeRoot.FindById("plugin-page-test-app3")
		require.NotNil(t, app3Node)
		require.Len(t, app3Node.Children, 1) // It should only have a single child now
		require.Equal(t, "Random page", app3Node.Children[0].Text)
	})
}

func TestReadingNavigationSettings(t *testing.T) {
	t.Run("Should include defaults", func(t *testing.T) {
		service := ServiceImpl{
			cfg: setting.NewCfg(),
		}

		_, _ = service.cfg.Raw.NewSection("navigation.app_sections")
		service.readNavigationSettings()

		require.Equal(t, "monitoring", service.navigationAppConfig["grafana-k8s-app"].SectionID)
	})

	t.Run("Can add additional overrides via ini system", func(t *testing.T) {
		service := ServiceImpl{
			cfg: setting.NewCfg(),
		}

		appSections, _ := service.cfg.Raw.NewSection("navigation.app_sections")
		appStandalonePages, _ := service.cfg.Raw.NewSection("navigation.app_standalone_pages")
		_, _ = appSections.NewKey("grafana-k8s-app", "dashboards")
		_, _ = appSections.NewKey("other-app", "admin 12")
		_, _ = appStandalonePages.NewKey("/a/grafana-k8s-app/foo", "admin 30")

		service.readNavigationSettings()

		require.Equal(t, "dashboards", service.navigationAppConfig["grafana-k8s-app"].SectionID)
		require.Equal(t, "admin", service.navigationAppConfig["other-app"].SectionID)

		require.Equal(t, int64(0), service.navigationAppConfig["grafana-k8s-app"].SortWeight)
		require.Equal(t, int64(12), service.navigationAppConfig["other-app"].SortWeight)

		require.Equal(t, "admin", service.navigationAppPathConfig["/a/grafana-k8s-app/foo"].SectionID)
		require.Equal(t, int64(30), service.navigationAppPathConfig["/a/grafana-k8s-app/foo"].SortWeight)
	})
}
