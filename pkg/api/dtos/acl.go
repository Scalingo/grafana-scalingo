package dtos

import "github.com/grafana/grafana/pkg/models"

// swagger:model
type UpdateDashboardAclCommand struct {
	Items []DashboardAclUpdateItem `json:"items"`
}

// swagger:model
type DashboardAclUpdateItem struct {
	UserID int64            `json:"userId"`
	TeamID int64            `json:"teamId"`
	Role   *models.RoleType `json:"role,omitempty"`
	// Permission level
	// Description:
	// * `1` - View
	// * `2` - Edit
	// * `4` - Admin
	// Enum: 1,2,4
	Permission models.PermissionType `json:"permission"`
}
