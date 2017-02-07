package middleware

import (
	"net/http"
	"strings"

	"github.com/grafana/grafana/pkg/metrics"
	"gopkg.in/macaron.v1"
)

func RequestMetrics() macaron.Handler {
	return func(res http.ResponseWriter, req *http.Request, c *macaron.Context) {
		rw := res.(macaron.ResponseWriter)
		c.Next()

		status := rw.Status()

		if strings.HasPrefix(req.RequestURI, "/api/datasources/proxy") {
			countProxyRequests(status)
		} else if strings.HasPrefix(req.RequestURI, "/api/") {
			countApiRequests(status)
		} else {
			countPageRequests(status)
		}
	}
}

func countApiRequests(status int) {
	switch status {
	case 200:
		metrics.M_Api_Status_200.Inc(1)
	case 404:
		metrics.M_Api_Status_404.Inc(1)
	case 500:
		metrics.M_Api_Status_500.Inc(1)
	default:
		metrics.M_Api_Status_Unknown.Inc(1)
	}
}

func countPageRequests(status int) {
	switch status {
	case 200:
		metrics.M_Page_Status_200.Inc(1)
	case 404:
		metrics.M_Page_Status_404.Inc(1)
	case 500:
		metrics.M_Page_Status_500.Inc(1)
	default:
		metrics.M_Page_Status_Unknown.Inc(1)
	}
}

func countProxyRequests(status int) {
	switch status {
	case 200:
		metrics.M_Proxy_Status_200.Inc(1)
	case 404:
		metrics.M_Proxy_Status_404.Inc(1)
	case 500:
		metrics.M_Proxy_Status_500.Inc(1)
	default:
		metrics.M_Proxy_Status_Unknown.Inc(1)
	}
}
