package ossaccesscontrol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/database"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

func setupTestEnv(t testing.TB) *OSSAccessControlService {
	t.Helper()

	ac := &OSSAccessControlService{
		features:      featuremgmt.WithFeatures(featuremgmt.FlagAccesscontrol),
		log:           log.New("accesscontrol"),
		registrations: accesscontrol.RegistrationList{},
		scopeResolver: accesscontrol.NewScopeResolver(),
		provider:      database.ProvideService(sqlstore.InitTestDB(t)),
		roles:         macroRoles(),
	}
	require.NoError(t, ac.RegisterFixedRoles())
	return ac
}

// extractRawPermissionsHelper extracts action and scope fields only from a permission slice
func extractRawPermissionsHelper(perms []*accesscontrol.Permission) []*accesscontrol.Permission {
	res := make([]*accesscontrol.Permission, len(perms))
	for i, p := range perms {
		res[i] = &accesscontrol.Permission{Action: p.Action, Scope: p.Scope}
	}
	return res
}

type evaluatingPermissionsTestCase struct {
	desc       string
	user       userTestCase
	endpoints  []endpointTestCase
	evalResult bool
}

type userTestCase struct {
	name           string
	orgRole        models.RoleType
	isGrafanaAdmin bool
}

type endpointTestCase struct {
	evaluator accesscontrol.Evaluator
}

func TestEvaluatingPermissions(t *testing.T) {
	testCases := []evaluatingPermissionsTestCase{
		{
			desc: "should successfully evaluate access to the endpoint",
			user: userTestCase{
				name:           "testuser",
				orgRole:        models.ROLE_VIEWER,
				isGrafanaAdmin: true,
			},
			endpoints: []endpointTestCase{
				{evaluator: accesscontrol.EvalPermission(accesscontrol.ActionUsersDisable, accesscontrol.ScopeGlobalUsersAll)},
				{evaluator: accesscontrol.EvalPermission(accesscontrol.ActionUsersEnable, accesscontrol.ScopeGlobalUsersAll)},
			},
			evalResult: true,
		},
		{
			desc: "should restrict access to the unauthorized endpoints",
			user: userTestCase{
				name:           "testuser",
				orgRole:        models.ROLE_VIEWER,
				isGrafanaAdmin: false,
			},
			endpoints: []endpointTestCase{
				{evaluator: accesscontrol.EvalPermission(accesscontrol.ActionUsersCreate, accesscontrol.ScopeGlobalUsersAll)},
			},
			evalResult: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ac := setupTestEnv(t)

			// Use OSS roles for this test to pass
			err := accesscontrol.DeclareFixedRoles(ac)
			require.NoError(t, err)

			errRegisterRoles := ac.RegisterFixedRoles()
			require.NoError(t, errRegisterRoles)

			user := &models.SignedInUser{
				UserId:         1,
				OrgId:          1,
				Name:           tc.user.name,
				OrgRole:        tc.user.orgRole,
				IsGrafanaAdmin: tc.user.isGrafanaAdmin,
			}

			for _, endpoint := range tc.endpoints {
				result, err := ac.Evaluate(context.Background(), user, endpoint.evaluator)
				require.NoError(t, err)
				assert.Equal(t, tc.evalResult, result)
			}
		})
	}
}

func TestUsageMetrics(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		expectedValue int
	}{
		{
			name:          "Expecting metric with value 0",
			enabled:       false,
			expectedValue: 0,
		},
		{
			name:          "Expecting metric with value 1",
			enabled:       true,
			expectedValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usagestatsmock := &usagestats.UsageStatsMock{T: t}

			_, errInitAc := ProvideService(
				featuremgmt.WithFeatures("accesscontrol", tt.enabled),
				usagestatsmock,
				database.ProvideService(sqlstore.InitTestDB(t)),
				routing.NewRouteRegister(),
			)
			require.NoError(t, errInitAc)
			report, err := usagestatsmock.GetUsageReport(context.Background())
			assert.Nil(t, err)

			assert.Equal(t, tt.expectedValue, report.Metrics["stats.oss.accesscontrol.enabled.count"])
		})
	}
}

func TestOSSAccessControlService_RegisterFixedRole(t *testing.T) {
	perm := accesscontrol.Permission{Action: "test:test", Scope: "test:*"}

	role := accesscontrol.RoleDTO{
		Version:     1,
		Name:        "fixed:test:test",
		Permissions: []accesscontrol.Permission{perm},
	}
	builtInRoles := []string{"Editor"}

	// Admin is going to get the role as well
	includedBuiltInRoles := []string{"Editor", "Admin"}

	// Grafana Admin and Viewer won't get the role
	excludedbuiltInRoles := []string{"Viewer", "Grafana Admin"}

	ac := setupTestEnv(t)
	ac.registerFixedRole(role, builtInRoles)

	for _, br := range includedBuiltInRoles {
		builtinRole, ok := ac.roles[br]
		assert.True(t, ok)
		assert.Contains(t, builtinRole.Permissions, perm)
	}

	for _, br := range excludedbuiltInRoles {
		builtinRole, ok := ac.roles[br]
		assert.True(t, ok)
		assert.NotContains(t, builtinRole.Permissions, perm)
	}
}

func TestOSSAccessControlService_DeclareFixedRoles(t *testing.T) {
	tests := []struct {
		name          string
		registrations []accesscontrol.RoleRegistration
		wantErr       bool
		err           error
	}{
		{
			name:    "should work with empty list",
			wantErr: false,
		},
		{
			name: "should add registration",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "fixed:test:test",
					},
					Grants: []string{"Admin"},
				},
			},
			wantErr: false,
		},
		{
			name: "should fail registration invalid role name",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "custom:test:test",
					},
					Grants: []string{"Admin"},
				},
			},
			wantErr: true,
			err:     accesscontrol.ErrFixedRolePrefixMissing,
		},
		{
			name: "should fail registration invalid builtin role assignment",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "fixed:test:test",
					},
					Grants: []string{"WrongAdmin"},
				},
			},
			wantErr: true,
			err:     accesscontrol.ErrInvalidBuiltinRole,
		},
		{
			name: "should add multiple registrations at once",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "fixed:test:test",
					},
					Grants: []string{"Admin"},
				},
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "fixed:test2:test2",
					},
					Grants: []string{"Admin"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := setupTestEnv(t)

			// Reset the registations
			ac.registrations = accesscontrol.RegistrationList{}

			// Test
			err := ac.DeclareFixedRoles(tt.registrations...)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)

			registrationCnt := 0
			ac.registrations.Range(func(registration accesscontrol.RoleRegistration) bool {
				registrationCnt++
				return true
			})
			assert.Equal(t, len(tt.registrations), registrationCnt,
				"expected service registration list to contain all test registrations")
		})
	}
}

func TestOSSAccessControlService_RegisterFixedRoles(t *testing.T) {
	tests := []struct {
		name          string
		token         models.Licensing
		registrations []accesscontrol.RoleRegistration
		wantErr       bool
	}{
		{
			name: "should work with empty list",
		},
		{
			name: "should register and assign role",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version:     1,
						Name:        "fixed:test:test",
						Permissions: []accesscontrol.Permission{{Action: "test:test"}},
					},
					Grants: []string{"Editor"},
				},
			},
			wantErr: false,
		},
		{
			name: "should register and assign multiple roles",
			registrations: []accesscontrol.RoleRegistration{
				{
					Role: accesscontrol.RoleDTO{
						Version:     1,
						Name:        "fixed:test:test",
						Permissions: []accesscontrol.Permission{{Action: "test:test"}},
					},
					Grants: []string{"Editor"},
				},
				{
					Role: accesscontrol.RoleDTO{
						Version: 1,
						Name:    "fixed:test2:test2",
						Permissions: []accesscontrol.Permission{
							{Action: "test:test2"},
							{Action: "test:test3", Scope: "test:*"},
						},
					},
					Grants: []string{"Viewer"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := setupTestEnv(t)

			ac.registrations.Append(tt.registrations...)

			// Test
			err := ac.RegisterFixedRoles()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Check
			for _, registration := range tt.registrations {
				// Prepare list of builtin roles to check
				brAndParents := map[string]struct{}{}
				for _, br := range registration.Grants {
					brAndParents[br] = struct{}{}
					if br != accesscontrol.RoleGrafanaAdmin {
						for _, parent := range models.RoleType(br).Parents() {
							brAndParents[string(parent)] = struct{}{}
						}
					}
				}

				// Check builtin roles (parents included) have been granted with the permissions
				for br := range brAndParents {
					builtinRole, ok := ac.roles[br]
					assert.True(t, ok)
					for _, expectedPermission := range registration.Role.Permissions {
						assert.Contains(t, builtinRole.Permissions, expectedPermission)
					}
				}
			}
		})
	}
}

func TestOSSAccessControlService_GetUserPermissions(t *testing.T) {
	testUser := models.SignedInUser{
		UserId:  2,
		OrgId:   3,
		OrgName: "TestOrg",
		OrgRole: models.ROLE_VIEWER,
		Login:   "testUser",
		Name:    "Test User",
		Email:   "testuser@example.org",
	}
	registration := accesscontrol.RoleRegistration{
		Role: accesscontrol.RoleDTO{
			Version:     1,
			UID:         "fixed:test:test",
			Name:        "fixed:test:test",
			Description: "Test role",
			Permissions: []accesscontrol.Permission{},
		},
		Grants: []string{"Viewer"},
	}
	tests := []struct {
		name     string
		user     models.SignedInUser
		rawPerm  accesscontrol.Permission
		wantPerm accesscontrol.Permission
		wantErr  bool
	}{
		{
			name:     "Translate users:self",
			user:     testUser,
			rawPerm:  accesscontrol.Permission{Action: "users:read", Scope: "users:self"},
			wantPerm: accesscontrol.Permission{Action: "users:read", Scope: "users:id:2"},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ac := setupTestEnv(t)

			registration.Role.Permissions = []accesscontrol.Permission{tt.rawPerm}
			err := ac.DeclareFixedRoles(registration)
			require.NoError(t, err)

			err = ac.RegisterFixedRoles()
			require.NoError(t, err)

			// Test
			userPerms, err := ac.GetUserPermissions(context.Background(), &tt.user, accesscontrol.Options{})
			if tt.wantErr {
				assert.Error(t, err, "Expected an error with GetUserPermissions.")
				return
			}
			require.NoError(t, err, "Did not expect an error with GetUserPermissions.")

			rawUserPerms := extractRawPermissionsHelper(userPerms)

			assert.Contains(t, rawUserPerms, &tt.wantPerm, "Expected resolution of raw permission")
			assert.NotContains(t, rawUserPerms, &tt.rawPerm, "Expected raw permission to have been resolved")
		})
	}
}

func TestOSSAccessControlService_Evaluate(t *testing.T) {
	testUser := models.SignedInUser{
		UserId:  2,
		OrgId:   3,
		OrgName: "TestOrg",
		OrgRole: models.ROLE_VIEWER,
		Login:   "testUser",
		Name:    "Test User",
		Email:   "testuser@example.org",
	}
	registration := accesscontrol.RoleRegistration{
		Role: accesscontrol.RoleDTO{
			Version:     1,
			UID:         "fixed:test:test",
			Name:        "fixed:test:test",
			Description: "Test role",
			Permissions: []accesscontrol.Permission{},
		},
		Grants: []string{"Viewer"},
	}
	userLoginScopeSolver := func(ctx context.Context, orgID int64, initialScope string) (string, error) {
		if initialScope == "users:login:testUser" {
			return "users:id:2", nil
		}
		return initialScope, nil
	}

	tests := []struct {
		name       string
		user       models.SignedInUser
		rawPerm    accesscontrol.Permission
		evaluator  accesscontrol.Evaluator
		wantAccess bool
		wantErr    bool
	}{
		{
			name:       "Should translate users:self",
			user:       testUser,
			rawPerm:    accesscontrol.Permission{Action: "users:read", Scope: "users:self"},
			evaluator:  accesscontrol.EvalPermission("users:read", "users:id:2"),
			wantAccess: true,
			wantErr:    false,
		},
		{
			name:       "Should translate users:login:testUser",
			user:       testUser,
			rawPerm:    accesscontrol.Permission{Action: "users:read", Scope: "users:id:2"},
			evaluator:  accesscontrol.EvalPermission("users:read", "users:login:testUser"),
			wantAccess: true,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ac := setupTestEnv(t)
			ac.RegisterAttributeScopeResolver("users:login:", userLoginScopeSolver)

			registration.Role.Permissions = []accesscontrol.Permission{tt.rawPerm}
			err := ac.DeclareFixedRoles(registration)
			require.NoError(t, err)

			err = ac.RegisterFixedRoles()
			require.NoError(t, err)

			// Test
			hasAccess, err := ac.Evaluate(context.TODO(), &tt.user, tt.evaluator)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			assert.Equal(t, tt.wantAccess, hasAccess)
		})
	}
}
