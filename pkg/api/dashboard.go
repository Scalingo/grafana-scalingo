package api

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/metrics"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/search"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

func isDashboardStarredByUser(c *middleware.Context, dashId int64) (bool, error) {
	if !c.IsSignedIn {
		return false, nil
	}

	query := m.IsStarredByUserQuery{UserId: c.UserId, DashboardId: dashId}
	if err := bus.Dispatch(&query); err != nil {
		return false, err
	}

	return query.Result, nil
}

func GetDashboard(c *middleware.Context) {
	slug := strings.ToLower(c.Params(":slug"))

	query := m.GetDashboardQuery{Slug: slug, OrgId: c.OrgId}
	err := bus.Dispatch(&query)
	if err != nil {
		c.JsonApiErr(404, "Dashboard not found", nil)
		return
	}

	isStarred, err := isDashboardStarredByUser(c, query.Result.Id)
	if err != nil {
		c.JsonApiErr(500, "Error while checking if dashboard was starred by user", err)
		return
	}

	dash := query.Result

	// Finding creator and last updater of the dashboard
	updater, creator := "Anonymous", "Anonymous"
	if dash.UpdatedBy > 0 {
		updater = getUserLogin(dash.UpdatedBy)
	}
	if dash.CreatedBy > 0 {
		creator = getUserLogin(dash.CreatedBy)
	}

	dto := dtos.DashboardFullWithMeta{
		Dashboard: dash.Data,
		Meta: dtos.DashboardMeta{
			IsStarred: isStarred,
			Slug:      slug,
			Type:      m.DashTypeDB,
			CanStar:   c.IsSignedIn,
			CanSave:   c.OrgRole == m.ROLE_ADMIN || c.OrgRole == m.ROLE_EDITOR,
			CanEdit:   canEditDashboard(c.OrgRole),
			Created:   dash.Created,
			Updated:   dash.Updated,
			UpdatedBy: updater,
			CreatedBy: creator,
			Version:   dash.Version,
		},
	}

	c.TimeRequest(metrics.M_Api_Dashboard_Get)
	c.JSON(200, dto)
}

func getUserLogin(userId int64) string {
	query := m.GetUserByIdQuery{Id: userId}
	err := bus.Dispatch(&query)
	if err != nil {
		return "Anonymous"
	} else {
		user := query.Result
		return user.Login
	}
}

func DeleteDashboard(c *middleware.Context) {
	slug := c.Params(":slug")

	query := m.GetDashboardQuery{Slug: slug, OrgId: c.OrgId}
	if err := bus.Dispatch(&query); err != nil {
		c.JsonApiErr(404, "Dashboard not found", nil)
		return
	}

	cmd := m.DeleteDashboardCommand{Slug: slug, OrgId: c.OrgId}
	if err := bus.Dispatch(&cmd); err != nil {
		c.JsonApiErr(500, "Failed to delete dashboard", err)
		return
	}

	var resp = map[string]interface{}{"title": query.Result.Title}

	c.JSON(200, resp)
}

func PostDashboard(c *middleware.Context, cmd m.SaveDashboardCommand) Response {
	cmd.OrgId = c.OrgId

	if !c.IsSignedIn {
		cmd.UserId = -1
	} else {
		cmd.UserId = c.UserId
	}

	dash := cmd.GetDashboardModel()
	// Check if Title is empty
	if dash.Title == "" {
		return ApiError(400, m.ErrDashboardTitleEmpty.Error(), nil)
	}
	if dash.Id == 0 {
		limitReached, err := middleware.QuotaReached(c, "dashboard")
		if err != nil {
			return ApiError(500, "failed to get quota", err)
		}
		if limitReached {
			return ApiError(403, "Quota reached", nil)
		}
	}

	validateAlertsCmd := alerting.ValidateDashboardAlertsCommand{
		OrgId:     c.OrgId,
		UserId:    c.UserId,
		Dashboard: dash,
	}

	if err := bus.Dispatch(&validateAlertsCmd); err != nil {
		return ApiError(500, "Invalid alert data. Cannot save dashboard", err)
	}

	err := bus.Dispatch(&cmd)
	if err != nil {
		if err == m.ErrDashboardWithSameNameExists {
			return Json(412, util.DynMap{"status": "name-exists", "message": err.Error()})
		}
		if err == m.ErrDashboardVersionMismatch {
			return Json(412, util.DynMap{"status": "version-mismatch", "message": err.Error()})
		}
		if pluginErr, ok := err.(m.UpdatePluginDashboardError); ok {
			message := "The dashboard belongs to plugin " + pluginErr.PluginId + "."
			// look up plugin name
			if pluginDef, exist := plugins.Plugins[pluginErr.PluginId]; exist {
				message = "The dashboard belongs to plugin " + pluginDef.Name + "."
			}
			return Json(412, util.DynMap{"status": "plugin-dashboard", "message": message})
		}
		if err == m.ErrDashboardNotFound {
			return Json(404, util.DynMap{"status": "not-found", "message": err.Error()})
		}
		return ApiError(500, "Failed to save dashboard", err)
	}

	alertCmd := alerting.UpdateDashboardAlertsCommand{
		OrgId:     c.OrgId,
		UserId:    c.UserId,
		Dashboard: cmd.Result,
	}

	if err := bus.Dispatch(&alertCmd); err != nil {
		return ApiError(500, "Failed to save alerts", err)
	}

	c.TimeRequest(metrics.M_Api_Dashboard_Save)
	return Json(200, util.DynMap{"status": "success", "slug": cmd.Result.Slug, "version": cmd.Result.Version})
}

func canEditDashboard(role m.RoleType) bool {
	return role == m.ROLE_ADMIN || role == m.ROLE_EDITOR || role == m.ROLE_READ_ONLY_EDITOR
}

func GetHomeDashboard(c *middleware.Context) Response {
	prefsQuery := m.GetPreferencesWithDefaultsQuery{OrgId: c.OrgId, UserId: c.UserId}
	if err := bus.Dispatch(&prefsQuery); err != nil {
		return ApiError(500, "Failed to get preferences", err)
	}

	if prefsQuery.Result.HomeDashboardId != 0 {
		slugQuery := m.GetDashboardSlugByIdQuery{Id: prefsQuery.Result.HomeDashboardId}
		err := bus.Dispatch(&slugQuery)
		if err == nil {
			dashRedirect := dtos.DashboardRedirect{RedirectUri: "db/" + slugQuery.Result}
			return Json(200, &dashRedirect)
		} else {
			log.Warn("Failed to get slug from database, %s", err.Error())
		}
	}

	filePath := path.Join(setting.StaticRootPath, "dashboards/home.json")
	file, err := os.Open(filePath)
	if err != nil {
		return ApiError(500, "Failed to load home dashboard", err)
	}

	dash := dtos.DashboardFullWithMeta{}
	dash.Meta.IsHome = true
	dash.Meta.CanEdit = canEditDashboard(c.OrgRole)
	jsonParser := json.NewDecoder(file)
	if err := jsonParser.Decode(&dash.Dashboard); err != nil {
		return ApiError(500, "Failed to load home dashboard", err)
	}

	if c.HasUserRole(m.ROLE_ADMIN) && !c.HasHelpFlag(m.HelpFlagGettingStartedPanelDismissed) {
		addGettingStartedPanelToHomeDashboard(dash.Dashboard)
	}

	return Json(200, &dash)
}

func addGettingStartedPanelToHomeDashboard(dash *simplejson.Json) {
	rows := dash.Get("rows").MustArray()
	row := simplejson.NewFromAny(rows[0])

	newpanel := simplejson.NewFromAny(map[string]interface{}{
		"type": "gettingstarted",
		"id":   123123,
		"span": 12,
	})

	panels := row.Get("panels").MustArray()
	panels = append(panels, newpanel)
	row.Set("panels", panels)
}

func GetDashboardFromJsonFile(c *middleware.Context) {
	file := c.Params(":file")

	dashboard := search.GetDashboardFromJsonIndex(file)
	if dashboard == nil {
		c.JsonApiErr(404, "Dashboard not found", nil)
		return
	}

	dash := dtos.DashboardFullWithMeta{Dashboard: dashboard.Data}
	dash.Meta.Type = m.DashTypeJson
	dash.Meta.CanEdit = canEditDashboard(c.OrgRole)

	c.JSON(200, &dash)
}

func GetDashboardTags(c *middleware.Context) {
	query := m.GetDashboardTagsQuery{OrgId: c.OrgId}
	err := bus.Dispatch(&query)
	if err != nil {
		c.JsonApiErr(500, "Failed to get tags from database", err)
		return
	}

	c.JSON(200, query.Result)
}
