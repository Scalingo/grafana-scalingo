package resourcepermissions

import (
	"context"

	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

type UidSolver func(ctx context.Context, orgID int64, uid string) (int64, error)
type ResourceValidator func(ctx context.Context, orgID int64, resourceID string) error
type InheritedScopesSolver func(ctx context.Context, orgID int64, resourceID string) ([]string, error)

type Options struct {
	// Resource is the action and scope prefix that is generated
	Resource string
	// ResourceAttribute is the attribute the scope should be based on (e.g. id or uid)
	ResourceAttribute string
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
	OnSetUser func(session *sqlstore.DBSession, orgID int64, user accesscontrol.User, resourceID, permission string) error
	// OnSetTeam if configured will be called each time a permission is set for a team
	OnSetTeam func(session *sqlstore.DBSession, orgID, teamID int64, resourceID, permission string) error
	// OnSetBuiltInRole if configured will be called each time a permission is set for a built-in role
	OnSetBuiltInRole func(session *sqlstore.DBSession, orgID int64, builtInRole, resourceID, permission string) error
	// UidSolver if configured will be used in a middleware to translate an uid to id for each request
	UidSolver UidSolver
	// InheritedScopesSolver if configured can generate additional scopes that will be used when fetching permissions for a resource
	InheritedScopesSolver InheritedScopesSolver
	// InheritedScopePrefixes if configured are used to create evaluators with the scopes returned by InheritedScopesSolver
	InheritedScopePrefixes []string
}
