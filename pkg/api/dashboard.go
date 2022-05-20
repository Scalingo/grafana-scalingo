package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/grafana/grafana/pkg/api/apierrors"
	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/components/dashdiffs"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/metrics"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

const (
	anonString = "Anonymous"
)

func (hs *HTTPServer) isDashboardStarredByUser(c *models.ReqContext, dashID int64) (bool, error) {
	if !c.IsSignedIn {
		return false, nil
	}

	query := models.IsStarredByUserQuery{UserId: c.UserId, DashboardId: dashID}
	if err := hs.SQLStore.IsStarredByUserCtx(c.Req.Context(), &query); err != nil {
		return false, err
	}

	return query.Result, nil
}

func dashboardGuardianResponse(err error) response.Response {
	if err != nil {
		return response.Error(500, "Error while checking dashboard permissions", err)
	}
	return response.Error(403, "Access denied to this dashboard", nil)
}

func (hs *HTTPServer) TrimDashboard(c *models.ReqContext) response.Response {
	cmd := models.TrimDashboardCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	var err error
	dash := cmd.Dashboard
	meta := cmd.Meta

	trimedResult := *dash
	if !hs.LoadSchemaService.IsDisabled() {
		trimedResult, err = hs.LoadSchemaService.DashboardTrimDefaults(*dash)
		if err != nil {
			return response.Error(500, "Error while exporting with default values removed", err)
		}
	}

	dto := dtos.TrimDashboardFullWithMeta{
		Dashboard: &trimedResult,
		Meta:      meta,
	}

	c.TimeRequest(metrics.MApiDashboardGet)
	return response.JSON(200, dto)
}

func (hs *HTTPServer) GetDashboard(c *models.ReqContext) response.Response {
	uid := web.Params(c.Req)[":uid"]
	dash, rsp := hs.getDashboardHelper(c.Req.Context(), c.OrgId, 0, uid)
	if rsp != nil {
		return rsp
	}
	// When dash contains only keys id, uid that means dashboard data is not valid and json decode failed.
	if dash.Data != nil {
		isEmptyData := true
		for k := range dash.Data.MustMap() {
			if k != "id" && k != "uid" {
				isEmptyData = false
				break
			}
		}
		if isEmptyData {
			return response.Error(500, "Error while loading dashboard, dashboard data is invalid", nil)
		}
	}
	guardian := guardian.New(c.Req.Context(), dash.Id, c.OrgId, c.SignedInUser)
	if canView, err := guardian.CanView(); err != nil || !canView {
		return dashboardGuardianResponse(err)
	}
	canEdit, _ := guardian.CanEdit()
	canSave, _ := guardian.CanSave()
	canAdmin, _ := guardian.CanAdmin()
	canDelete, _ := guardian.CanDelete()

	isStarred, err := hs.isDashboardStarredByUser(c, dash.Id)
	if err != nil {
		return response.Error(500, "Error while checking if dashboard was starred by user", err)
	}
	// Finding creator and last updater of the dashboard
	updater, creator := anonString, anonString
	if dash.UpdatedBy > 0 {
		updater = hs.getUserLogin(c.Req.Context(), dash.UpdatedBy)
	}
	if dash.CreatedBy > 0 {
		creator = hs.getUserLogin(c.Req.Context(), dash.CreatedBy)
	}

	annotationPermissions := &dtos.AnnotationPermission{}

	if !hs.AccessControl.IsDisabled() {
		hs.getAnnotationPermissionsByScope(c, &annotationPermissions.Dashboard, accesscontrol.ScopeAnnotationsTypeDashboard)
		hs.getAnnotationPermissionsByScope(c, &annotationPermissions.Organization, accesscontrol.ScopeAnnotationsTypeOrganization)
	}

	meta := dtos.DashboardMeta{
		IsStarred:              isStarred,
		Slug:                   dash.Slug,
		Type:                   models.DashTypeDB,
		CanStar:                c.IsSignedIn,
		CanSave:                canSave,
		CanEdit:                canEdit,
		CanAdmin:               canAdmin,
		CanDelete:              canDelete,
		Created:                dash.Created,
		Updated:                dash.Updated,
		UpdatedBy:              updater,
		CreatedBy:              creator,
		Version:                dash.Version,
		HasAcl:                 dash.HasAcl,
		IsFolder:               dash.IsFolder,
		FolderId:               dash.FolderId,
		Url:                    dash.GetUrl(),
		FolderTitle:            "General",
		AnnotationsPermissions: annotationPermissions,
	}

	// lookup folder title
	if dash.FolderId > 0 {
		query := models.GetDashboardQuery{Id: dash.FolderId, OrgId: c.OrgId}
		if err := hs.SQLStore.GetDashboard(c.Req.Context(), &query); err != nil {
			if errors.Is(err, models.ErrFolderNotFound) {
				return response.Error(404, "Folder not found", err)
			}
			return response.Error(500, "Dashboard folder could not be read", err)
		}
		meta.FolderUid = query.Result.Uid
		meta.FolderTitle = query.Result.Title
		meta.FolderUrl = query.Result.GetUrl()
	}

	provisioningData, err := hs.dashboardProvisioningService.GetProvisionedDashboardDataByDashboardID(dash.Id)
	if err != nil {
		return response.Error(500, "Error while checking if dashboard is provisioned", err)
	}

	if provisioningData != nil {
		allowUIUpdate := hs.ProvisioningService.GetAllowUIUpdatesFromConfig(provisioningData.Name)
		if !allowUIUpdate {
			meta.Provisioned = true
		}

		meta.ProvisionedExternalId, err = filepath.Rel(
			hs.ProvisioningService.GetDashboardProvisionerResolvedPath(provisioningData.Name),
			provisioningData.ExternalId,
		)
		if err != nil {
			// Not sure when this could happen so not sure how to better handle this. Right now ProvisionedExternalId
			// is for better UX, showing in Save/Delete dialogs and so it won't break anything if it is empty.
			hs.log.Warn("Failed to create ProvisionedExternalId", "err", err)
		}
	}

	// make sure db version is in sync with json model version
	dash.Data.Set("version", dash.Version)

	// load library panels JSON for this dashboard
	err = hs.LibraryPanelService.LoadLibraryPanelsForDashboard(c.Req.Context(), dash)
	if err != nil {
		return response.Error(500, "Error while loading library panels", err)
	}

	dto := dtos.DashboardFullWithMeta{
		Dashboard: dash.Data,
		Meta:      meta,
	}

	c.TimeRequest(metrics.MApiDashboardGet)
	return response.JSON(200, dto)
}

func (hs *HTTPServer) getAnnotationPermissionsByScope(c *models.ReqContext, actions *dtos.AnnotationActions, scope string) {
	var err error

	evaluate := accesscontrol.EvalPermission(accesscontrol.ActionAnnotationsCreate, scope)
	actions.CanAdd, err = hs.AccessControl.Evaluate(c.Req.Context(), c.SignedInUser, evaluate)
	if err != nil {
		hs.log.Warn("Failed to evaluate permission", "err", err, "action", accesscontrol.ActionAnnotationsCreate, "scope", scope)
	}

	evaluate = accesscontrol.EvalPermission(accesscontrol.ActionAnnotationsDelete, scope)
	actions.CanDelete, err = hs.AccessControl.Evaluate(c.Req.Context(), c.SignedInUser, evaluate)
	if err != nil {
		hs.log.Warn("Failed to evaluate permission", "err", err, "action", accesscontrol.ActionAnnotationsDelete, "scope", scope)
	}

	evaluate = accesscontrol.EvalPermission(accesscontrol.ActionAnnotationsWrite, scope)
	actions.CanEdit, err = hs.AccessControl.Evaluate(c.Req.Context(), c.SignedInUser, evaluate)
	if err != nil {
		hs.log.Warn("Failed to evaluate permission", "err", err, "action", accesscontrol.ActionAnnotationsWrite, "scope", scope)
	}
}

func (hs *HTTPServer) getUserLogin(ctx context.Context, userID int64) string {
	query := models.GetUserByIdQuery{Id: userID}
	err := hs.SQLStore.GetUserById(ctx, &query)
	if err != nil {
		return anonString
	}
	return query.Result.Login
}

func (hs *HTTPServer) getDashboardHelper(ctx context.Context, orgID int64, id int64, uid string) (*models.Dashboard, response.Response) {
	var query models.GetDashboardQuery

	if len(uid) > 0 {
		query = models.GetDashboardQuery{Uid: uid, Id: id, OrgId: orgID}
	} else {
		query = models.GetDashboardQuery{Id: id, OrgId: orgID}
	}

	if err := hs.SQLStore.GetDashboard(ctx, &query); err != nil {
		return nil, response.Error(404, "Dashboard not found", err)
	}

	return query.Result, nil
}

func (hs *HTTPServer) DeleteDashboardByUID(c *models.ReqContext) response.Response {
	return hs.deleteDashboard(c)
}

func (hs *HTTPServer) deleteDashboard(c *models.ReqContext) response.Response {
	dash, rsp := hs.getDashboardHelper(c.Req.Context(), c.OrgId, 0, web.Params(c.Req)[":uid"])
	if rsp != nil {
		return rsp
	}
	guardian := guardian.New(c.Req.Context(), dash.Id, c.OrgId, c.SignedInUser)
	if canDelete, err := guardian.CanDelete(); err != nil || !canDelete {
		return dashboardGuardianResponse(err)
	}

	// disconnect all library elements for this dashboard
	err := hs.LibraryElementService.DisconnectElementsFromDashboard(c.Req.Context(), dash.Id)
	if err != nil {
		hs.log.Error("Failed to disconnect library elements", "dashboard", dash.Id, "user", c.SignedInUser.UserId, "error", err)
	}

	err = hs.dashboardService.DeleteDashboard(c.Req.Context(), dash.Id, c.OrgId)
	if err != nil {
		var dashboardErr models.DashboardErr
		if ok := errors.As(err, &dashboardErr); ok {
			if errors.Is(err, models.ErrDashboardCannotDeleteProvisionedDashboard) {
				return response.Error(dashboardErr.StatusCode, dashboardErr.Error(), err)
			}
		}
		return response.Error(500, "Failed to delete dashboard", err)
	}
	if hs.Live != nil {
		err := hs.Live.GrafanaScope.Dashboards.DashboardDeleted(c.OrgId, c.ToUserDisplayDTO(), dash.Uid)
		if err != nil {
			hs.log.Error("Failed to broadcast delete info", "dashboard", dash.Uid, "error", err)
		}
	}
	return response.JSON(200, util.DynMap{
		"title":   dash.Title,
		"message": fmt.Sprintf("Dashboard %s deleted", dash.Title),
		"id":      dash.Id,
	})
}

func (hs *HTTPServer) PostDashboard(c *models.ReqContext) response.Response {
	cmd := models.SaveDashboardCommand{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return hs.postDashboard(c, cmd)
}

func (hs *HTTPServer) postDashboard(c *models.ReqContext, cmd models.SaveDashboardCommand) response.Response {
	ctx := c.Req.Context()
	var err error
	cmd.OrgId = c.OrgId
	cmd.UserId = c.UserId
	if cmd.FolderUid != "" {
		folder, err := hs.folderService.GetFolderByUID(ctx, c.SignedInUser, c.OrgId, cmd.FolderUid)
		if err != nil {
			if errors.Is(err, models.ErrFolderNotFound) {
				return response.Error(400, "Folder not found", err)
			}
			return response.Error(500, "Error while checking folder ID", err)
		}
		cmd.FolderId = folder.Id
	}

	dash := cmd.GetDashboardModel()
	newDashboard := dash.Id == 0
	if newDashboard {
		limitReached, err := hs.QuotaService.QuotaReached(c, "dashboard")
		if err != nil {
			return response.Error(500, "failed to get quota", err)
		}
		if limitReached {
			return response.Error(403, "Quota reached", nil)
		}
	}

	var provisioningData *models.DashboardProvisioning
	if dash.Id != 0 {
		data, err := hs.dashboardProvisioningService.GetProvisionedDashboardDataByDashboardID(dash.Id)
		if err != nil {
			return response.Error(500, "Error while checking if dashboard is provisioned using ID", err)
		}
		provisioningData = data
	} else if dash.Uid != "" {
		data, err := hs.dashboardProvisioningService.GetProvisionedDashboardDataByDashboardUID(dash.OrgId, dash.Uid)
		if err != nil && !errors.Is(err, models.ErrProvisionedDashboardNotFound) && !errors.Is(err, models.ErrDashboardNotFound) {
			return response.Error(500, "Error while checking if dashboard is provisioned", err)
		}
		provisioningData = data
	}

	allowUiUpdate := true
	if provisioningData != nil {
		allowUiUpdate = hs.ProvisioningService.GetAllowUIUpdatesFromConfig(provisioningData.Name)
	}

	// clean up all unnecessary library panels JSON properties so we store a minimum JSON
	err = hs.LibraryPanelService.CleanLibraryPanelsForDashboard(dash)
	if err != nil {
		return response.Error(500, "Error while cleaning library panels", err)
	}

	dashItem := &dashboards.SaveDashboardDTO{
		Dashboard: dash,
		Message:   cmd.Message,
		OrgId:     c.OrgId,
		User:      c.SignedInUser,
		Overwrite: cmd.Overwrite,
	}

	dashboard, err := hs.dashboardService.SaveDashboard(alerting.WithUAEnabled(ctx, hs.Cfg.UnifiedAlerting.IsEnabled()), dashItem, allowUiUpdate)

	if hs.Live != nil {
		// Tell everyone listening that the dashboard changed
		if dashboard == nil {
			dashboard = dash // the original request
		}

		// This will broadcast all save requests only if a `gitops` observer exists.
		// gitops is useful when trying to save dashboards in an environment where the user can not save
		channel := hs.Live.GrafanaScope.Dashboards
		liveerr := channel.DashboardSaved(c.SignedInUser.OrgId, c.SignedInUser.ToUserDisplayDTO(), cmd.Message, dashboard, err)

		// When an error exists, but the value broadcast to a gitops listener return 202
		if liveerr == nil && err != nil && channel.HasGitOpsObserver(c.SignedInUser.OrgId) {
			return response.JSON(202, util.DynMap{
				"status":  "pending",
				"message": "changes were broadcast to the gitops listener",
			})
		}

		if liveerr != nil {
			hs.log.Warn("unable to broadcast save event", "uid", dashboard.Uid, "error", err)
		}
	}

	if err != nil {
		return apierrors.ToDashboardErrorResponse(ctx, hs.pluginStore, err)
	}

	// connect library panels for this dashboard after the dashboard is stored and has an ID
	err = hs.LibraryPanelService.ConnectLibraryPanelsForDashboard(ctx, c.SignedInUser, dashboard)
	if err != nil {
		return response.Error(500, "Error while connecting library panels", err)
	}

	c.TimeRequest(metrics.MApiDashboardSave)
	return response.JSON(200, util.DynMap{
		"status":  "success",
		"slug":    dashboard.Slug,
		"version": dashboard.Version,
		"id":      dashboard.Id,
		"uid":     dashboard.Uid,
		"url":     dashboard.GetUrl(),
	})
}

// GetHomeDashboard returns the home dashboard.
func (hs *HTTPServer) GetHomeDashboard(c *models.ReqContext) response.Response {
	prefsQuery := models.GetPreferencesWithDefaultsQuery{User: c.SignedInUser}
	homePage := hs.Cfg.HomePage

	if err := hs.SQLStore.GetPreferencesWithDefaults(c.Req.Context(), &prefsQuery); err != nil {
		return response.Error(500, "Failed to get preferences", err)
	}

	if prefsQuery.Result.HomeDashboardId == 0 && len(homePage) > 0 {
		homePageRedirect := dtos.DashboardRedirect{RedirectUri: homePage}
		return response.JSON(200, &homePageRedirect)
	}

	if prefsQuery.Result.HomeDashboardId != 0 {
		slugQuery := models.GetDashboardRefByIdQuery{Id: prefsQuery.Result.HomeDashboardId}
		err := hs.SQLStore.GetDashboardUIDById(c.Req.Context(), &slugQuery)
		if err == nil {
			url := models.GetDashboardUrl(slugQuery.Result.Uid, slugQuery.Result.Slug)
			dashRedirect := dtos.DashboardRedirect{RedirectUri: url}
			return response.JSON(200, &dashRedirect)
		}
		hs.log.Warn("Failed to get slug from database", "err", err)
	}

	filePath := hs.Cfg.DefaultHomeDashboardPath
	if filePath == "" {
		filePath = filepath.Join(hs.Cfg.StaticRootPath, "dashboards/home.json")
	}

	// It's safe to ignore gosec warning G304 since the variable part of the file path comes from a configuration
	// variable
	// nolint:gosec
	file, err := os.Open(filePath)
	if err != nil {
		return response.Error(500, "Failed to load home dashboard", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			hs.log.Warn("Failed to close dashboard file", "path", filePath, "err", err)
		}
	}()

	dash := dtos.DashboardFullWithMeta{}
	dash.Meta.IsHome = true
	dash.Meta.CanEdit = c.SignedInUser.HasRole(models.ROLE_EDITOR)
	dash.Meta.FolderTitle = "General"
	dash.Dashboard = simplejson.New()

	jsonParser := json.NewDecoder(file)
	if err := jsonParser.Decode(dash.Dashboard); err != nil {
		return response.Error(500, "Failed to load home dashboard", err)
	}

	hs.addGettingStartedPanelToHomeDashboard(c, dash.Dashboard)

	return response.JSON(200, &dash)
}

func (hs *HTTPServer) addGettingStartedPanelToHomeDashboard(c *models.ReqContext, dash *simplejson.Json) {
	// We only add this getting started panel for Admins who have not dismissed it,
	// and if a custom default home dashboard hasn't been configured
	if !c.HasUserRole(models.ROLE_ADMIN) ||
		c.HasHelpFlag(models.HelpFlagGettingStartedPanelDismissed) ||
		hs.Cfg.DefaultHomeDashboardPath != "" {
		return
	}

	panels := dash.Get("panels").MustArray()

	newpanel := simplejson.NewFromAny(map[string]interface{}{
		"type": "gettingstarted",
		"id":   123123,
		"gridPos": map[string]interface{}{
			"x": 0,
			"y": 3,
			"w": 24,
			"h": 9,
		},
	})

	panels = append(panels, newpanel)
	dash.Set("panels", panels)
}

// GetDashboardVersions returns all dashboard versions as JSON
func (hs *HTTPServer) GetDashboardVersions(c *models.ReqContext) response.Response {
	dashID, err := strconv.ParseInt(web.Params(c.Req)[":dashboardId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "dashboardId is invalid", err)
	}

	guardian := guardian.New(c.Req.Context(), dashID, c.OrgId, c.SignedInUser)
	if canSave, err := guardian.CanSave(); err != nil || !canSave {
		return dashboardGuardianResponse(err)
	}

	query := models.GetDashboardVersionsQuery{
		OrgId:       c.OrgId,
		DashboardId: dashID,
		Limit:       c.QueryInt("limit"),
		Start:       c.QueryInt("start"),
	}

	if err := hs.SQLStore.GetDashboardVersions(c.Req.Context(), &query); err != nil {
		return response.Error(404, fmt.Sprintf("No versions found for dashboardId %d", dashID), err)
	}

	for _, version := range query.Result {
		if version.RestoredFrom == version.Version {
			version.Message = "Initial save (created by migration)"
			continue
		}

		if version.RestoredFrom > 0 {
			version.Message = fmt.Sprintf("Restored from version %d", version.RestoredFrom)
			continue
		}

		if version.ParentVersion == 0 {
			version.Message = "Initial save"
		}
	}

	return response.JSON(200, query.Result)
}

// GetDashboardVersion returns the dashboard version with the given ID.
func (hs *HTTPServer) GetDashboardVersion(c *models.ReqContext) response.Response {
	dashID, err := strconv.ParseInt(web.Params(c.Req)[":dashboardId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "dashboardId is invalid", err)
	}

	guardian := guardian.New(c.Req.Context(), dashID, c.OrgId, c.SignedInUser)
	if canSave, err := guardian.CanSave(); err != nil || !canSave {
		return dashboardGuardianResponse(err)
	}

	version, _ := strconv.ParseInt(web.Params(c.Req)[":id"], 10, 32)
	query := models.GetDashboardVersionQuery{
		OrgId:       c.OrgId,
		DashboardId: dashID,
		Version:     int(version),
	}

	if err := hs.SQLStore.GetDashboardVersion(c.Req.Context(), &query); err != nil {
		return response.Error(500, fmt.Sprintf("Dashboard version %d not found for dashboardId %d", query.Version, dashID), err)
	}

	creator := anonString
	if query.Result.CreatedBy > 0 {
		creator = hs.getUserLogin(c.Req.Context(), query.Result.CreatedBy)
	}

	dashVersionMeta := &models.DashboardVersionMeta{
		Id:            query.Result.Id,
		DashboardId:   query.Result.DashboardId,
		Data:          query.Result.Data,
		ParentVersion: query.Result.ParentVersion,
		RestoredFrom:  query.Result.RestoredFrom,
		Version:       query.Result.Version,
		Created:       query.Result.Created,
		Message:       query.Result.Message,
		CreatedBy:     creator,
	}

	return response.JSON(200, dashVersionMeta)
}

// POST /api/dashboards/calculate-diff performs diffs on two dashboards
func (hs *HTTPServer) CalculateDashboardDiff(c *models.ReqContext) response.Response {
	apiOptions := dtos.CalculateDiffOptions{}
	if err := web.Bind(c.Req, &apiOptions); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	guardianBase := guardian.New(c.Req.Context(), apiOptions.Base.DashboardId, c.OrgId, c.SignedInUser)
	if canSave, err := guardianBase.CanSave(); err != nil || !canSave {
		return dashboardGuardianResponse(err)
	}

	if apiOptions.Base.DashboardId != apiOptions.New.DashboardId {
		guardianNew := guardian.New(c.Req.Context(), apiOptions.New.DashboardId, c.OrgId, c.SignedInUser)
		if canSave, err := guardianNew.CanSave(); err != nil || !canSave {
			return dashboardGuardianResponse(err)
		}
	}

	options := dashdiffs.Options{
		OrgId:    c.OrgId,
		DiffType: dashdiffs.ParseDiffType(apiOptions.DiffType),
		Base: dashdiffs.DiffTarget{
			DashboardId:      apiOptions.Base.DashboardId,
			Version:          apiOptions.Base.Version,
			UnsavedDashboard: apiOptions.Base.UnsavedDashboard,
		},
		New: dashdiffs.DiffTarget{
			DashboardId:      apiOptions.New.DashboardId,
			Version:          apiOptions.New.Version,
			UnsavedDashboard: apiOptions.New.UnsavedDashboard,
		},
	}

	baseVersionQuery := models.GetDashboardVersionQuery{
		DashboardId: options.Base.DashboardId,
		Version:     options.Base.Version,
		OrgId:       options.OrgId,
	}

	if err := hs.SQLStore.GetDashboardVersion(c.Req.Context(), &baseVersionQuery); err != nil {
		if errors.Is(err, models.ErrDashboardVersionNotFound) {
			return response.Error(404, "Dashboard version not found", err)
		}
		return response.Error(500, "Unable to compute diff", err)
	}

	newVersionQuery := models.GetDashboardVersionQuery{
		DashboardId: options.New.DashboardId,
		Version:     options.New.Version,
		OrgId:       options.OrgId,
	}

	if err := hs.SQLStore.GetDashboardVersion(c.Req.Context(), &newVersionQuery); err != nil {
		if errors.Is(err, models.ErrDashboardVersionNotFound) {
			return response.Error(404, "Dashboard version not found", err)
		}
		return response.Error(500, "Unable to compute diff", err)
	}

	baseData := baseVersionQuery.Result.Data
	newData := newVersionQuery.Result.Data

	result, err := dashdiffs.CalculateDiff(c.Req.Context(), &options, baseData, newData)

	if err != nil {
		if errors.Is(err, models.ErrDashboardVersionNotFound) {
			return response.Error(404, "Dashboard version not found", err)
		}
		return response.Error(500, "Unable to compute diff", err)
	}

	if options.DiffType == dashdiffs.DiffDelta {
		return response.Respond(200, result.Delta).SetHeader("Content-Type", "application/json")
	}

	return response.Respond(200, result.Delta).SetHeader("Content-Type", "text/html")
}

// RestoreDashboardVersion restores a dashboard to the given version.
func (hs *HTTPServer) RestoreDashboardVersion(c *models.ReqContext) response.Response {
	apiCmd := dtos.RestoreDashboardVersionCommand{}
	if err := web.Bind(c.Req, &apiCmd); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	dashboardId, err := strconv.ParseInt(web.Params(c.Req)[":dashboardId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "dashboardId is invalid", err)
	}

	dash, rsp := hs.getDashboardHelper(c.Req.Context(), c.OrgId, dashboardId, "")
	if rsp != nil {
		return rsp
	}

	guardian := guardian.New(c.Req.Context(), dash.Id, c.OrgId, c.SignedInUser)
	if canSave, err := guardian.CanSave(); err != nil || !canSave {
		return dashboardGuardianResponse(err)
	}

	versionQuery := models.GetDashboardVersionQuery{DashboardId: dash.Id, Version: apiCmd.Version, OrgId: c.OrgId}
	if err := hs.SQLStore.GetDashboardVersion(c.Req.Context(), &versionQuery); err != nil {
		return response.Error(404, "Dashboard version not found", nil)
	}

	version := versionQuery.Result

	saveCmd := models.SaveDashboardCommand{}
	saveCmd.RestoredFrom = version.Version
	saveCmd.OrgId = c.OrgId
	saveCmd.UserId = c.UserId
	saveCmd.Dashboard = version.Data
	saveCmd.Dashboard.Set("version", dash.Version)
	saveCmd.Dashboard.Set("uid", dash.Uid)
	saveCmd.Message = fmt.Sprintf("Restored from version %d", version.Version)
	saveCmd.FolderId = dash.FolderId

	return hs.postDashboard(c, saveCmd)
}

func (hs *HTTPServer) GetDashboardTags(c *models.ReqContext) {
	query := models.GetDashboardTagsQuery{OrgId: c.OrgId}
	err := hs.SQLStore.GetDashboardTags(c.Req.Context(), &query)
	if err != nil {
		c.JsonApiErr(500, "Failed to get tags from database", err)
		return
	}

	c.JSON(200, query.Result)
}
