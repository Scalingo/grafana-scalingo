package proxyutil

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/stretchr/testify/require"
)

func TestReverseProxy(t *testing.T) {
	t.Run("When proxying a request should enforce request and response constraints", func(t *testing.T) {
		var actualReq *http.Request
		upstream := newUpstreamServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			actualReq = req
			http.SetCookie(w, &http.Cookie{Name: "test"})
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(upstream.Close)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)
		req.Header.Set("X-Forwarded-Host", "forwarded.host.com")
		req.Header.Set("X-Forwarded-Port", "8080")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Origin", "test.com")
		req.Header.Set("Referer", "https://test.com/api")
		req.RemoteAddr = "10.0.0.1"

		rp := NewReverseProxy(log.New("test"), func(req *http.Request) {
			req.Header.Set("X-KEY", "value")
		})
		require.NotNil(t, rp)
		require.NotNil(t, rp.ModifyResponse)
		rp.ServeHTTP(rec, req)

		require.NotNil(t, actualReq)
		require.Empty(t, actualReq.Header.Get("X-Forwarded-Host"))
		require.Empty(t, actualReq.Header.Get("X-Forwarded-Port"))
		require.Empty(t, actualReq.Header.Get("X-Forwarded-Proto"))
		require.Equal(t, "10.0.0.1", actualReq.Header.Get("X-Forwarded-For"))
		require.Empty(t, actualReq.Header.Get("Origin"))
		require.Empty(t, actualReq.Header.Get("Referer"))
		require.Equal(t, "value", actualReq.Header.Get("X-KEY"))
		resp := rec.Result()
		require.Empty(t, resp.Cookies())
		require.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))
		require.NoError(t, resp.Body.Close())
	})

	t.Run("When proxying a request using WithModifyResponse should call it before default ModifyResponse func", func(t *testing.T) {
		var actualReq *http.Request
		upstream := newUpstreamServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			actualReq = req
			http.SetCookie(w, &http.Cookie{Name: "test"})
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(upstream.Close)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)
		rp := NewReverseProxy(
			log.New("test"),
			func(req *http.Request) {
				req.Header.Set("X-KEY", "value")
			},
			WithModifyResponse(func(r *http.Response) error {
				r.Header.Set("X-KEY2", "value2")
				return nil
			}),
		)
		require.NotNil(t, rp)
		require.NotNil(t, rp.ModifyResponse)
		rp.ServeHTTP(rec, req)

		require.NotNil(t, actualReq)
		require.Equal(t, "value", actualReq.Header.Get("X-KEY"))
		resp := rec.Result()
		require.Empty(t, resp.Cookies())
		require.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))
		require.Equal(t, "value2", resp.Header.Get("X-KEY2"))
		require.NoError(t, resp.Body.Close())
	})

	t.Run("Error handling should convert status codes depending on what kind of error it is", func(t *testing.T) {
		timedOutTransport := http.DefaultTransport.(*http.Transport)
		timedOutTransport.ResponseHeaderTimeout = time.Millisecond

		testCases := []struct {
			desc               string
			transport          http.RoundTripper
			responseWaitTime   time.Duration
			expectedStatusCode int
		}{
			{
				desc:               "Cancelled request should return 499 Client closed request",
				transport:          &cancelledRoundTripper{},
				expectedStatusCode: StatusClientClosedRequest,
			},
			{
				desc:               "Timed out request should return 504 Gateway timeout",
				transport:          timedOutTransport,
				responseWaitTime:   100 * time.Millisecond,
				expectedStatusCode: http.StatusGatewayTimeout,
			},
			{
				desc:               "Failed request should return 502 Bad gateway",
				transport:          &failingRoundTripper{},
				expectedStatusCode: http.StatusBadGateway,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				upstream := newUpstreamServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if tc.responseWaitTime > 0 {
						time.Sleep(tc.responseWaitTime)
					}

					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(upstream.Close)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, upstream.URL, nil)

				rp := NewReverseProxy(
					log.New("test"),
					func(req *http.Request) {},
					WithTransport(tc.transport),
				)
				require.NotNil(t, rp)
				require.NotNil(t, rp.Transport)
				require.Same(t, tc.transport, rp.Transport)
				rp.ServeHTTP(rec, req)

				resp := rec.Result()
				require.Equal(t, tc.expectedStatusCode, resp.StatusCode)
				require.NoError(t, resp.Body.Close())
			})
		}
	})
}

func newUpstreamServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	upstream := httptest.NewServer(handler)
	return upstream
}

type cancelledRoundTripper struct{}

func (cancelledRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.Canceled
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("some error")
}
