package resourcepermissions

import (
	"context"
)

type ResourceValidator func(ctx context.Context, orgID int64, resourceID string) error

type Options struct {
	// Resource is the action and scope prefix that is generated
	Resource string
	// OnlyManaged will tell the service to return all permissions if set to false and only managed permissions if set to true
	OnlyManaged bool
	// ResourceValidator is a validator function that will be called before each assignment.
	// If set to nil the validator will be skipped
	ResourceValidator ResourceValidator
	// Assignments decides what we can assign permissions to (users/teams/builtInRoles)
	Assignments Assignments
	// PermissionsToAction is a map of friendly named permissions and what access control actions they should generate.
	// E.g. Edit permissions should generate dashboards:read, dashboards:write and dashboards:delete
	PermissionsToActions map[string][]string
	// ReaderRoleName is the display name for the generated fixed reader role
	ReaderRoleName string
	// WriterRoleName is the display name for the generated fixed writer role
	WriterRoleName string
	// RoleGroup is the group name for the generated fixed roles
	RoleGroup string
	// OnSetUser if configured will be called each time a permission is set for a user
	OnSetUser func(ctx context.Context, orgID, userID int64, resourceID, permission string) error
	// OnSetTeam if configured will be called each time a permission is set for a team
	OnSetTeam func(ctx context.Context, orgID, teamID int64, resourceID, permission string) error
	// OnSetBuiltInRole if configured will be called each time a permission is set for a built-in role
	OnSetBuiltInRole func(ctx context.Context, orgID int64, builtInRole, resourceID, permission string) error
}
