package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"

	macaron "gopkg.in/macaron.v1"

	"github.com/grafana/grafana/pkg/api/live"
	httpstatic "github.com/grafana/grafana/pkg/api/static"
	"github.com/grafana/grafana/pkg/cmd/grafana-cli/logger"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/setting"
)

type HttpServer struct {
	log           log.Logger
	macaron       *macaron.Macaron
	context       context.Context
	streamManager *live.StreamManager

	httpSrv *http.Server
}

func NewHttpServer() *HttpServer {
	return &HttpServer{
		log: log.New("http.server"),
	}
}

func (hs *HttpServer) Start(ctx context.Context) error {
	var err error

	hs.context = ctx
	hs.streamManager = live.NewStreamManager()
	hs.macaron = hs.newMacaron()
	hs.registerRoutes()

	hs.streamManager.Run(ctx)

	listenAddr := fmt.Sprintf("%s:%s", setting.HttpAddr, setting.HttpPort)
	hs.log.Info("Initializing HTTP Server", "address", listenAddr, "protocol", setting.Protocol, "subUrl", setting.AppSubUrl)

	hs.httpSrv = &http.Server{Addr: listenAddr, Handler: hs.macaron}
	switch setting.Protocol {
	case setting.HTTP:
		err = hs.httpSrv.ListenAndServe()
		if err == http.ErrServerClosed {
			hs.log.Debug("server was shutdown gracefully")
			return nil
		}
	case setting.HTTPS:
		err = hs.httpSrv.ListenAndServeTLS(setting.CertFile, setting.KeyFile)
		if err == http.ErrServerClosed {
			hs.log.Debug("server was shutdown gracefully")
			return nil
		}
	default:
		hs.log.Error("Invalid protocol", "protocol", setting.Protocol)
		err = errors.New("Invalid Protocol")
	}

	return err
}

func (hs *HttpServer) Shutdown(ctx context.Context) error {
	err := hs.httpSrv.Shutdown(ctx)
	hs.log.Info("stopped http server")
	return err
}

func (hs *HttpServer) listenAndServeTLS(listenAddr, certfile, keyfile string) error {
	if certfile == "" {
		return fmt.Errorf("cert_file cannot be empty when using HTTPS")
	}

	if keyfile == "" {
		return fmt.Errorf("cert_key cannot be empty when using HTTPS")
	}

	if _, err := os.Stat(setting.CertFile); os.IsNotExist(err) {
		return fmt.Errorf(`Cannot find SSL cert_file at %v`, setting.CertFile)
	}

	if _, err := os.Stat(setting.KeyFile); os.IsNotExist(err) {
		return fmt.Errorf(`Cannot find SSL key_file at %v`, setting.KeyFile)
	}

	tlsCfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      hs.macaron,
		TLSConfig:    tlsCfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	return srv.ListenAndServeTLS(setting.CertFile, setting.KeyFile)
}

func (hs *HttpServer) newMacaron() *macaron.Macaron {
	macaron.Env = setting.Env
	m := macaron.New()

	m.Use(middleware.Logger())
	m.Use(middleware.Recovery())

	if setting.EnableGzip {
		m.Use(middleware.Gziper())
	}

	for _, route := range plugins.StaticRoutes {
		pluginRoute := path.Join("/public/plugins/", route.PluginId)
		logger.Debug("Plugins: Adding route", "route", pluginRoute, "dir", route.Directory)
		hs.mapStatic(m, route.Directory, "", pluginRoute)
	}

	hs.mapStatic(m, setting.StaticRootPath, "", "public")
	hs.mapStatic(m, setting.StaticRootPath, "robots.txt", "robots.txt")

	m.Use(macaron.Renderer(macaron.RenderOptions{
		Directory:  path.Join(setting.StaticRootPath, "views"),
		IndentJSON: macaron.Env != macaron.PROD,
		Delims:     macaron.Delims{Left: "[[", Right: "]]"},
	}))

	m.Use(middleware.GetContextHandler())
	m.Use(middleware.Sessioner(&setting.SessionOptions))
	m.Use(middleware.RequestMetrics())
	m.Use(middleware.OrgRedirect())

	// needs to be after context handler
	if setting.EnforceDomain {
		m.Use(middleware.ValidateHostHeader(setting.Domain))
	}

	return m
}

func (hs *HttpServer) mapStatic(m *macaron.Macaron, rootDir string, dir string, prefix string) {
	headers := func(c *macaron.Context) {
		c.Resp.Header().Set("Cache-Control", "public, max-age=3600")
	}

	if setting.Env == setting.DEV {
		headers = func(c *macaron.Context) {
			c.Resp.Header().Set("Cache-Control", "max-age=0, must-revalidate, no-cache")
		}
	}

	m.Use(httpstatic.Static(
		path.Join(rootDir, dir),
		httpstatic.StaticOptions{
			SkipLogging: true,
			Prefix:      prefix,
			AddHeaders:  headers,
		},
	))
}
