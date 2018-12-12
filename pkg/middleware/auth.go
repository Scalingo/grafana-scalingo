package middleware

import (
	"net/url"
	"strings"

	"gopkg.in/macaron.v1"

	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/session"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

type AuthOptions struct {
	ReqGrafanaAdmin bool
	ReqSignedIn     bool
}

func getRequestUserId(c *m.ReqContext) int64 {
	userID := c.Session.Get(session.SESS_KEY_USERID)

	if userID != nil {
		return userID.(int64)
	}

	return 0
}

func getApiKey(c *m.ReqContext) string {
	header := c.Req.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		key := parts[1]
		return key
	}

	username, password, err := util.DecodeBasicAuthHeader(header)
	if err == nil && username == "api_key" {
		return password
	}

	return ""
}

func accessForbidden(c *m.ReqContext) {
	if c.IsApiRequest() {
		c.JsonApiErr(403, "Permission denied", nil)
		return
	}

	c.Redirect(setting.AppSubUrl + "/")
}

func notAuthorized(c *m.ReqContext) {
	if c.IsApiRequest() {
		c.JsonApiErr(401, "Unauthorized", nil)
		return
	}

	c.SetCookie("redirect_to", url.QueryEscape(setting.AppSubUrl+c.Req.RequestURI), 0, setting.AppSubUrl+"/", nil, false, true)

	c.Redirect(setting.AppSubUrl + "/login")
}

func RoleAuth(roles ...m.RoleType) macaron.Handler {
	return func(c *m.ReqContext) {
		ok := false
		for _, role := range roles {
			if role == c.OrgRole {
				ok = true
				break
			}
		}
		if !ok {
			accessForbidden(c)
		}
	}
}

func Auth(options *AuthOptions) macaron.Handler {
	return func(c *m.ReqContext) {
		if !c.IsSignedIn && options.ReqSignedIn && !c.AllowAnonymous {
			notAuthorized(c)
			return
		}

		if !c.IsGrafanaAdmin && options.ReqGrafanaAdmin {
			accessForbidden(c)
			return
		}
	}
}
