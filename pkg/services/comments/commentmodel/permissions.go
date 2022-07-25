package commentmodel

import (
	"context"
	"strconv"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/annotations"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

type PermissionChecker struct {
	sqlStore         *sqlstore.SQLStore
	features         featuremgmt.FeatureToggles
	accessControl    accesscontrol.AccessControl
	dashboardService dashboards.DashboardService
}

func NewPermissionChecker(sqlStore *sqlstore.SQLStore, features featuremgmt.FeatureToggles,
	accessControl accesscontrol.AccessControl, dashboardService dashboards.DashboardService,
) *PermissionChecker {
	return &PermissionChecker{sqlStore: sqlStore, features: features, accessControl: accessControl}
}

func (c *PermissionChecker) getDashboardByUid(ctx context.Context, orgID int64, uid string) (*models.Dashboard, error) {
	query := models.GetDashboardQuery{Uid: uid, OrgId: orgID}
	if err := c.dashboardService.GetDashboard(ctx, &query); err != nil {
		return nil, err
	}
	return query.Result, nil
}

func (c *PermissionChecker) getDashboardById(ctx context.Context, orgID int64, id int64) (*models.Dashboard, error) {
	query := models.GetDashboardQuery{Id: id, OrgId: orgID}
	if err := c.dashboardService.GetDashboard(ctx, &query); err != nil {
		return nil, err
	}
	return query.Result, nil
}

func (c *PermissionChecker) CheckReadPermissions(ctx context.Context, orgId int64, signedInUser *models.SignedInUser, objectType string, objectID string) (bool, error) {
	switch objectType {
	case ObjectTypeOrg:
		return false, nil
	case ObjectTypeDashboard:
		if !c.features.IsEnabled(featuremgmt.FlagDashboardComments) {
			return false, nil
		}
		dash, err := c.getDashboardByUid(ctx, orgId, objectID)
		if err != nil {
			return false, err
		}
		guard := guardian.New(ctx, dash.Id, orgId, signedInUser)
		if ok, err := guard.CanView(); err != nil || !ok {
			return false, nil
		}
	case ObjectTypeAnnotation:
		if !c.features.IsEnabled(featuremgmt.FlagAnnotationComments) {
			return false, nil
		}
		repo := annotations.GetRepository()
		annotationID, err := strconv.ParseInt(objectID, 10, 64)
		if err != nil {
			return false, nil
		}
		items, err := repo.Find(ctx, &annotations.ItemQuery{AnnotationId: annotationID, OrgId: orgId, SignedInUser: signedInUser})
		if err != nil || len(items) != 1 {
			return false, nil
		}
		dashboardID := items[0].DashboardId
		if dashboardID == 0 {
			return false, nil
		}
		dash, err := c.getDashboardById(ctx, orgId, dashboardID)
		if err != nil {
			return false, err
		}
		guard := guardian.New(ctx, dash.Id, orgId, signedInUser)
		if ok, err := guard.CanView(); err != nil || !ok {
			return false, nil
		}
	default:
		return false, nil
	}
	return true, nil
}

func (c *PermissionChecker) CheckWritePermissions(ctx context.Context, orgId int64, signedInUser *models.SignedInUser, objectType string, objectID string) (bool, error) {
	switch objectType {
	case ObjectTypeOrg:
		return false, nil
	case ObjectTypeDashboard:
		if !c.features.IsEnabled(featuremgmt.FlagDashboardComments) {
			return false, nil
		}
		dash, err := c.getDashboardByUid(ctx, orgId, objectID)
		if err != nil {
			return false, err
		}
		guard := guardian.New(ctx, dash.Id, orgId, signedInUser)
		if ok, err := guard.CanEdit(); err != nil || !ok {
			return false, nil
		}
	case ObjectTypeAnnotation:
		if !c.features.IsEnabled(featuremgmt.FlagAnnotationComments) {
			return false, nil
		}
		if !c.accessControl.IsDisabled() {
			evaluator := accesscontrol.EvalPermission(accesscontrol.ActionAnnotationsWrite, accesscontrol.ScopeAnnotationsTypeDashboard)
			if canEdit, err := c.accessControl.Evaluate(ctx, signedInUser, evaluator); err != nil || !canEdit {
				return canEdit, err
			}
		}
		repo := annotations.GetRepository()
		annotationID, err := strconv.ParseInt(objectID, 10, 64)
		if err != nil {
			return false, nil
		}
		items, err := repo.Find(ctx, &annotations.ItemQuery{AnnotationId: annotationID, OrgId: orgId, SignedInUser: signedInUser})
		if err != nil || len(items) != 1 {
			return false, nil
		}
		dashboardID := items[0].DashboardId
		if dashboardID == 0 {
			return false, nil
		}
		dash, err := c.getDashboardById(ctx, orgId, dashboardID)
		if err != nil {
			return false, nil
		}
		guard := guardian.New(ctx, dash.Id, orgId, signedInUser)
		if ok, err := guard.CanEdit(); err != nil || !ok {
			return false, nil
		}
	default:
		return false, nil
	}
	return true, nil
}
