package api

import (
	"net/http"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
)

type AccessControlAPI struct {
	RouteRegister routing.RouteRegister
	AccessControl ac.AccessControl
}

func (api *AccessControlAPI) RegisterAPIEndpoints() {
	// Users
	api.RouteRegister.Get("/api/access-control/user/permissions",
		middleware.ReqSignedIn, routing.Wrap(api.getUsersPermissions))
}

// GET /api/access-control/user/permissions
func (api *AccessControlAPI) getUsersPermissions(c *models.ReqContext) response.Response {
	reloadCache := c.QueryBool("reloadcache")
	permissions, err := api.AccessControl.GetUserPermissions(c.Req.Context(),
		c.SignedInUser, ac.Options{ReloadCache: reloadCache})
	if err != nil {
		response.JSON(http.StatusInternalServerError, err)
	}

	return response.JSON(http.StatusOK, ac.BuildPermissionsMap(permissions))
}
