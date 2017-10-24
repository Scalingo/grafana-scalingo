package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/metrics"
	"gopkg.in/macaron.v1"
)

func RequestMetrics(handler string) macaron.Handler {
	return func(res http.ResponseWriter, req *http.Request, c *macaron.Context) {
		rw := res.(macaron.ResponseWriter)
		now := time.Now()
		c.Next()

		status := rw.Status()

		code := sanitizeCode(status)
		method := sanitizeMethod(req.Method)
		metrics.M_Http_Request_Total.WithLabelValues(handler, code, method).Inc()
		duration := time.Since(now).Nanoseconds() / int64(time.Millisecond)
		metrics.M_Http_Request_Summary.WithLabelValues(handler, code, method).Observe(float64(duration))

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
		metrics.M_Api_Status.WithLabelValues("200").Inc()
	case 404:
		metrics.M_Api_Status.WithLabelValues("404").Inc()
	case 500:
		metrics.M_Api_Status.WithLabelValues("500").Inc()
	default:
		metrics.M_Api_Status.WithLabelValues("unknown").Inc()
	}
}

func countPageRequests(status int) {
	switch status {
	case 200:
		metrics.M_Page_Status.WithLabelValues("200").Inc()
	case 404:
		metrics.M_Page_Status.WithLabelValues("404").Inc()
	case 500:
		metrics.M_Page_Status.WithLabelValues("500").Inc()
	default:
		metrics.M_Page_Status.WithLabelValues("unknown").Inc()
	}
}

func countProxyRequests(status int) {
	switch status {
	case 200:
		metrics.M_Proxy_Status.WithLabelValues("200").Inc()
	case 404:
		metrics.M_Proxy_Status.WithLabelValues("400").Inc()
	case 500:
		metrics.M_Proxy_Status.WithLabelValues("500").Inc()
	default:
		metrics.M_Proxy_Status.WithLabelValues("unknown").Inc()
	}
}

func sanitizeMethod(m string) string {
	switch m {
	case "GET", "get":
		return "get"
	case "PUT", "put":
		return "put"
	case "HEAD", "head":
		return "head"
	case "POST", "post":
		return "post"
	case "DELETE", "delete":
		return "delete"
	case "CONNECT", "connect":
		return "connect"
	case "OPTIONS", "options":
		return "options"
	case "NOTIFY", "notify":
		return "notify"
	default:
		return strings.ToLower(m)
	}
}

// If the wrapped http.Handler has not set a status code, i.e. the value is
// currently 0, santizeCode will return 200, for consistency with behavior in
// the stdlib.
func sanitizeCode(s int) string {
	switch s {
	case 100:
		return "100"
	case 101:
		return "101"

	case 200, 0:
		return "200"
	case 201:
		return "201"
	case 202:
		return "202"
	case 203:
		return "203"
	case 204:
		return "204"
	case 205:
		return "205"
	case 206:
		return "206"

	case 300:
		return "300"
	case 301:
		return "301"
	case 302:
		return "302"
	case 304:
		return "304"
	case 305:
		return "305"
	case 307:
		return "307"

	case 400:
		return "400"
	case 401:
		return "401"
	case 402:
		return "402"
	case 403:
		return "403"
	case 404:
		return "404"
	case 405:
		return "405"
	case 406:
		return "406"
	case 407:
		return "407"
	case 408:
		return "408"
	case 409:
		return "409"
	case 410:
		return "410"
	case 411:
		return "411"
	case 412:
		return "412"
	case 413:
		return "413"
	case 414:
		return "414"
	case 415:
		return "415"
	case 416:
		return "416"
	case 417:
		return "417"
	case 418:
		return "418"

	case 500:
		return "500"
	case 501:
		return "501"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	case 505:
		return "505"

	case 428:
		return "428"
	case 429:
		return "429"
	case 431:
		return "431"
	case 511:
		return "511"

	default:
		return strconv.Itoa(s)
	}
}
