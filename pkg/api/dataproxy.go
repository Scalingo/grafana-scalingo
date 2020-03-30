package api

import (
	"github.com/grafana/grafana/pkg/api/pluginproxy"
	"github.com/grafana/grafana/pkg/infra/metrics"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
)

// ProxyDataSourceRequest proxies datasource requests
func (hs *HTTPServer) ProxyDataSourceRequest(c *models.ReqContext) {
	c.TimeRequest(metrics.MDataSourceProxyReqTimer)

	dsID := c.ParamsInt64(":id")
	ds, err := hs.DatasourceCache.GetDatasource(dsID, c.SignedInUser, c.SkipCache)
	if err != nil {
		if err == models.ErrDataSourceAccessDenied {
			c.JsonApiErr(403, "Access denied to datasource", err)
			return
		}
		c.JsonApiErr(500, "Unable to load datasource meta data", err)
		return
	}

	// find plugin
	plugin, ok := plugins.DataSources[ds.Type]
	if !ok {
		c.JsonApiErr(500, "Unable to find datasource plugin", err)
		return
	}

	// macaron does not include trailing slashes when resolving a wildcard path
	proxyPath := ensureProxyPathTrailingSlash(c.Req.URL.Path, c.Params("*"))

	proxy := pluginproxy.NewDataSourceProxy(ds, plugin, c, proxyPath, hs.Cfg)
	proxy.HandleRequest()
}

// ensureProxyPathTrailingSlash Check for a trailing slash in original path and makes
// sure that a trailing slash is added to proxy path, if not already exists.
func ensureProxyPathTrailingSlash(originalPath, proxyPath string) string {
	if len(proxyPath) > 1 {
		if originalPath[len(originalPath)-1] == '/' && proxyPath[len(proxyPath)-1] != '/' {
			return proxyPath + "/"
		}
	}

	return proxyPath
}
