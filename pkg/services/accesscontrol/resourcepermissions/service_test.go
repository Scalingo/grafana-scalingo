package resourcepermissions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/database"
	accesscontrolmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
)

type setUserPermissionTest struct {
	desc     string
	callHook bool
}

func TestService_SetUserPermission(t *testing.T) {
	tests := []setUserPermissionTest{
		{
			desc:     "should call hook when updating user permissions",
			callHook: true,
		},
		{
			desc:     "should not call hook when updating user permissions",
			callHook: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service, sql := setupTestEnvironment(t, []*accesscontrol.Permission{}, Options{
				Resource:             "dashboards",
				Assignments:          Assignments{Users: true},
				PermissionsToActions: nil,
			})

			// seed user
			user, err := sql.CreateUser(context.Background(), models.CreateUserCommand{Login: "test", OrgId: 1})
			require.NoError(t, err)

			var hookCalled bool
			if tt.callHook {
				service.options.OnSetUser = func(session *sqlstore.DBSession, orgID int64, user accesscontrol.User, resourceID, permission string) error {
					hookCalled = true
					return nil
				}
			}

			_, err = service.SetUserPermission(context.Background(), user.OrgId, accesscontrol.User{ID: user.Id}, "1", "")
			require.NoError(t, err)
			assert.Equal(t, tt.callHook, hookCalled)
		})
	}
}

type setTeamPermissionTest struct {
	desc     string
	callHook bool
}

func TestService_SetTeamPermission(t *testing.T) {
	tests := []setTeamPermissionTest{
		{
			desc:     "should call hook when updating user permissions",
			callHook: true,
		},
		{
			desc:     "should not call hook when updating user permissions",
			callHook: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service, sql := setupTestEnvironment(t, []*accesscontrol.Permission{}, Options{
				Resource:             "dashboards",
				Assignments:          Assignments{Teams: true},
				PermissionsToActions: nil,
			})

			// seed team
			team, err := sql.CreateTeam("test", "test@test.com", 1)
			require.NoError(t, err)

			var hookCalled bool
			if tt.callHook {
				service.options.OnSetTeam = func(session *sqlstore.DBSession, orgID, teamID int64, resourceID, permission string) error {
					hookCalled = true
					return nil
				}
			}

			_, err = service.SetTeamPermission(context.Background(), team.OrgId, team.Id, "1", "")
			require.NoError(t, err)
			assert.Equal(t, tt.callHook, hookCalled)
		})
	}
}

type setBuiltInRolePermissionTest struct {
	desc     string
	callHook bool
}

func TestService_SetBuiltInRolePermission(t *testing.T) {
	tests := []setBuiltInRolePermissionTest{
		{
			desc:     "should call hook when updating user permissions",
			callHook: true,
		},
		{
			desc:     "should not call hook when updating user permissions",
			callHook: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service, _ := setupTestEnvironment(t, []*accesscontrol.Permission{}, Options{
				Resource:             "dashboards",
				Assignments:          Assignments{BuiltInRoles: true},
				PermissionsToActions: nil,
			})

			var hookCalled bool
			if tt.callHook {
				service.options.OnSetBuiltInRole = func(session *sqlstore.DBSession, orgID int64, builtInRole, resourceID, permission string) error {
					hookCalled = true
					return nil
				}
			}

			_, err := service.SetBuiltInRolePermission(context.Background(), 1, "Viewer", "1", "")
			require.NoError(t, err)
			assert.Equal(t, tt.callHook, hookCalled)
		})
	}
}

type setPermissionsTest struct {
	desc      string
	options   Options
	commands  []accesscontrol.SetResourcePermissionCommand
	expectErr bool
}

func TestService_SetPermissions(t *testing.T) {
	tests := []setPermissionsTest{
		{
			desc: "should set all permissions",
			options: Options{
				Resource: "dashboards",
				Assignments: Assignments{
					Users:        true,
					Teams:        true,
					BuiltInRoles: true,
				},
				PermissionsToActions: map[string][]string{
					"View": {"dashboards:read"},
				},
			},
			commands: []accesscontrol.SetResourcePermissionCommand{
				{UserID: 1, Permission: "View"},
				{TeamID: 1, Permission: "View"},
				{BuiltinRole: "Editor", Permission: "View"},
			},
		},
		{
			desc: "should return error for invalid permission",
			options: Options{
				Resource: "dashboards",
				Assignments: Assignments{
					Users:        true,
					Teams:        true,
					BuiltInRoles: true,
				},
				PermissionsToActions: map[string][]string{
					"View": {"dashboards:read"},
				},
			},
			commands: []accesscontrol.SetResourcePermissionCommand{
				{UserID: 1, Permission: "View"},
				{TeamID: 1, Permission: "View"},
				{BuiltinRole: "Editor", Permission: "Not real permission"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			service, sql := setupTestEnvironment(t, []*accesscontrol.Permission{}, tt.options)

			// seed user
			_, err := sql.CreateUser(context.Background(), models.CreateUserCommand{Login: "user", OrgId: 1})
			require.NoError(t, err)
			_, err = sql.CreateTeam("team", "", 1)
			require.NoError(t, err)

			permissions, err := service.SetPermissions(context.Background(), 1, "1", tt.commands...)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, permissions, len(tt.commands))
			}
		})
	}
}

func setupTestEnvironment(t *testing.T, permissions []*accesscontrol.Permission, ops Options) (*Service, *sqlstore.SQLStore) {
	t.Helper()

	sql := sqlstore.InitTestDB(t)
	store := database.ProvideService(sql)
	cfg := setting.NewCfg()
	cfg.IsEnterprise = true
	service, err := New(ops, cfg, routing.NewRouteRegister(), accesscontrolmock.New().WithPermissions(permissions), store, sql)
	require.NoError(t, err)

	return service, sql
}
