package middleware

import (
	"fmt"
	"strings"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

var (
	ReqGrafanaAdmin = Auth(&AuthOptions{
		ReqSignedIn:     true,
		ReqGrafanaAdmin: true,
	})
	ReqSignedIn            = Auth(&AuthOptions{ReqSignedIn: true})
	ReqSignedInNoAnonymous = Auth(&AuthOptions{ReqSignedIn: true, ReqNoAnonynmous: true})
	ReqEditorRole          = RoleAuth(models.ROLE_EDITOR, models.ROLE_ADMIN)
	ReqOrgAdmin            = RoleAuth(models.ROLE_ADMIN)
)

func HandleNoCacheHeader(ctx *models.ReqContext) {
	ctx.SkipCache = ctx.Req.Header.Get("X-Grafana-NoCache") == "true"
}

func AddDefaultResponseHeaders(cfg *setting.Cfg) web.Handler {
	return func(c *web.Context) {
		c.Resp.Before(func(w web.ResponseWriter) {
			// if response has already been written, skip.
			if w.Written() {
				return
			}

			if !strings.HasPrefix(c.Req.URL.Path, "/api/datasources/proxy/") {
				addNoCacheHeaders(c.Resp)
			}

			if !cfg.AllowEmbedding {
				addXFrameOptionsDenyHeader(w)
			}

			addSecurityHeaders(w, cfg)
		})
	}
}

// addSecurityHeaders adds HTTP(S) response headers that enable various security protections in the client's browser.
func addSecurityHeaders(w web.ResponseWriter, cfg *setting.Cfg) {
	if (cfg.Protocol == setting.HTTPSScheme || cfg.Protocol == setting.HTTP2Scheme) && cfg.StrictTransportSecurity {
		strictHeaderValues := []string{fmt.Sprintf("max-age=%v", cfg.StrictTransportSecurityMaxAge)}
		if cfg.StrictTransportSecurityPreload {
			strictHeaderValues = append(strictHeaderValues, "preload")
		}
		if cfg.StrictTransportSecuritySubDomains {
			strictHeaderValues = append(strictHeaderValues, "includeSubDomains")
		}
		w.Header().Set("Strict-Transport-Security", strings.Join(strictHeaderValues, "; "))
	}

	if cfg.ContentTypeProtectionHeader {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}

	if cfg.XSSProtectionHeader {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	}
}

func addNoCacheHeaders(w web.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "-1")
}

func addXFrameOptionsDenyHeader(w web.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "deny")
}
