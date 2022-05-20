package api

import (
	"github.com/grafana/grafana/pkg/models"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/serviceaccounts"
	"github.com/grafana/grafana/pkg/setting"
)

// API related actions
const (
	ActionProvisioningReload = "provisioning:reload"

	ActionOrgsRead             = "orgs:read"
	ActionOrgsPreferencesRead  = "orgs.preferences:read"
	ActionOrgsQuotasRead       = "orgs.quotas:read"
	ActionOrgsWrite            = "orgs:write"
	ActionOrgsPreferencesWrite = "orgs.preferences:write"
	ActionOrgsQuotasWrite      = "orgs.quotas:write"
	ActionOrgsDelete           = "orgs:delete"
	ActionOrgsCreate           = "orgs:create"
)

// API related scopes
var (
	ScopeProvisionersAll           = ac.Scope("provisioners", "*")
	ScopeProvisionersDashboards    = ac.Scope("provisioners", "dashboards")
	ScopeProvisionersPlugins       = ac.Scope("provisioners", "plugins")
	ScopeProvisionersDatasources   = ac.Scope("provisioners", "datasources")
	ScopeProvisionersNotifications = ac.Scope("provisioners", "notifications")
)

// declareFixedRoles declares to the AccessControl service fixed roles and their
// grants to organization roles ("Viewer", "Editor", "Admin") or "Grafana Admin"
// that HTTPServer needs
func (hs *HTTPServer) declareFixedRoles() error {
	provisioningWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     3,
			Name:        "fixed:provisioning:writer",
			DisplayName: "Provisioning writer",
			Description: "Reload provisioning.",
			Group:       "Provisioning",
			Permissions: []ac.Permission{
				{
					Action: ActionProvisioningReload,
					Scope:  ScopeProvisionersAll,
				},
			},
		},
		Grants: []string{ac.RoleGrafanaAdmin},
	}

	datasourcesExplorerRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     4,
			Name:        "fixed:datasources:explorer",
			DisplayName: "Data source explorer",
			Description: "Enable the Explore feature. Data source permissions still apply; you can only query data sources for which you have query permissions.",
			Group:       "Data sources",
			Permissions: []ac.Permission{
				{
					Action: ac.ActionDatasourcesExplore,
				},
			},
		},
		Grants: []string{string(models.ROLE_EDITOR)},
	}

	if setting.ViewersCanEdit {
		datasourcesExplorerRole.Grants = append(datasourcesExplorerRole.Grants, string(models.ROLE_VIEWER))
	}

	datasourcesReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     3,
			Name:        "fixed:datasources:reader",
			DisplayName: "Data source reader",
			Description: "Read and query all data sources.",
			Group:       "Data sources",
			Permissions: []ac.Permission{
				{
					Action: datasources.ActionRead,
					Scope:  datasources.ScopeAll,
				},
				{
					Action: datasources.ActionQuery,
					Scope:  datasources.ScopeAll,
				},
			},
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	datasourcesWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     3,
			Name:        "fixed:datasources:writer",
			DisplayName: "Data source writer",
			Description: "Create, update, delete, read, or query data sources.",
			Group:       "Data sources",
			Permissions: ac.ConcatPermissions(datasourcesReaderRole.Role.Permissions, []ac.Permission{
				{
					Action: datasources.ActionWrite,
					Scope:  datasources.ScopeAll,
				},
				{
					Action: datasources.ActionCreate,
				},
				{
					Action: datasources.ActionDelete,
					Scope:  datasources.ScopeAll,
				},
			}),
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	datasourcesIdReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     4,
			Name:        "fixed:datasources.id:reader",
			DisplayName: "Data source ID reader",
			Description: "Read the ID of a data source based on its name.",
			Group:       "Infrequently used",
			Permissions: []ac.Permission{
				{
					Action: datasources.ActionIDRead,
					Scope:  datasources.ScopeAll,
				},
			},
		},
		Grants: []string{string(models.ROLE_VIEWER)},
	}

	datasourcesCompatibilityReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     3,
			Name:        "fixed:datasources:compatibility:querier",
			DisplayName: "Data source compatibility querier",
			Description: "Only used for open source compatibility. Query data sources.",
			Group:       "Infrequently used",
			Permissions: []ac.Permission{
				{Action: datasources.ActionQuery},
				{Action: datasources.ActionRead},
			},
		},
		Grants: []string{string(models.ROLE_VIEWER)},
	}

	apikeyReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:apikeys:reader",
			DisplayName: "APIKeys reader",
			Description: "Gives access to read api keys.",
			Group:       "API Keys",
			Permissions: []ac.Permission{
				{
					Action: ac.ActionAPIKeyRead,
					Scope:  ac.ScopeAPIKeysAll,
				},
			},
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	apikeyWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:apikeys:writer",
			DisplayName: "APIKeys writer",
			Description: "Gives access to add and delete api keys.",
			Group:       "API Keys",
			Permissions: ac.ConcatPermissions(apikeyReaderRole.Role.Permissions, []ac.Permission{
				{
					Action: ac.ActionAPIKeyCreate,
				},
				{
					Action: ac.ActionAPIKeyDelete,
					Scope:  ac.ScopeAPIKeysAll,
				},
			}),
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	orgReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     5,
			Name:        "fixed:organization:reader",
			DisplayName: "Organization reader",
			Description: "Read an organization, such as its ID, name, address, or quotas.",
			Group:       "Organizations",
			Permissions: []ac.Permission{
				{Action: ActionOrgsRead},
				{Action: ActionOrgsQuotasRead},
			},
		},
		Grants: []string{string(models.ROLE_VIEWER), ac.RoleGrafanaAdmin},
	}

	orgWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     5,
			Name:        "fixed:organization:writer",
			DisplayName: "Organization writer",
			Description: "Read an organization, its quotas, or its preferences. Update organization properties, or its preferences.",
			Group:       "Organizations",
			Permissions: ac.ConcatPermissions(orgReaderRole.Role.Permissions, []ac.Permission{
				{Action: ActionOrgsPreferencesRead},
				{Action: ActionOrgsWrite},
				{Action: ActionOrgsPreferencesWrite},
			}),
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	orgMaintainerRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     5,
			Name:        "fixed:organization:maintainer",
			DisplayName: "Organization maintainer",
			Description: "Create, read, write, or delete an organization. Read or write an organization's quotas. Needs to be assigned globally.",
			Group:       "Organizations",
			Permissions: ac.ConcatPermissions(orgReaderRole.Role.Permissions, []ac.Permission{
				{Action: ActionOrgsCreate},
				{Action: ActionOrgsWrite},
				{Action: ActionOrgsDelete},
				{Action: ActionOrgsQuotasWrite},
			}),
		},
		Grants: []string{string(ac.RoleGrafanaAdmin)},
	}

	teamCreatorGrants := []string{string(models.ROLE_ADMIN)}
	if hs.Cfg.EditorsCanAdmin {
		teamCreatorGrants = append(teamCreatorGrants, string(models.ROLE_EDITOR))
	}
	teamsCreatorRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Name:        "fixed:teams:creator",
			DisplayName: "Team creator",
			Description: "Create teams and read organisation users (required to manage the created teams).",
			Group:       "Teams",
			Version:     2,
			Permissions: []ac.Permission{
				{Action: ac.ActionTeamsCreate},
				{Action: ac.ActionOrgUsersRead, Scope: ac.ScopeUsersAll},
			},
		},
		Grants: teamCreatorGrants,
	}

	teamsWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Name:        "fixed:teams:writer",
			DisplayName: "Team writer",
			Description: "Create, read, write, or delete a team as well as controlling team memberships.",
			Group:       "Teams",
			Version:     2,
			Permissions: []ac.Permission{
				{Action: ac.ActionTeamsCreate},
				{Action: ac.ActionTeamsDelete, Scope: ac.ScopeTeamsAll},
				{Action: ac.ActionTeamsPermissionsRead, Scope: ac.ScopeTeamsAll},
				{Action: ac.ActionTeamsPermissionsWrite, Scope: ac.ScopeTeamsAll},
				{Action: ac.ActionTeamsRead, Scope: ac.ScopeTeamsAll},
				{Action: ac.ActionTeamsWrite, Scope: ac.ScopeTeamsAll},
			},
		},
		Grants: []string{string(models.ROLE_ADMIN)},
	}

	annotationsReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Name:        "fixed:annotations:reader",
			DisplayName: "Annotation reader",
			Description: "Read annotations and tags",
			Group:       "Annotations",
			Version:     2,
			Permissions: []ac.Permission{
				{Action: ac.ActionAnnotationsRead, Scope: ac.ScopeAnnotationsAll},
			},
		},
		Grants: []string{string(models.ROLE_VIEWER)},
	}

	dashboardAnnotationsWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Name:        "fixed:annotations.dashboard:writer",
			DisplayName: "Dashboard annotation writer",
			Description: "Update annotations associated with dashboards.",
			Group:       "Annotations",
			Version:     3,
			Permissions: []ac.Permission{
				{Action: ac.ActionAnnotationsCreate, Scope: ac.ScopeAnnotationsTypeDashboard},
				{Action: ac.ActionAnnotationsDelete, Scope: ac.ScopeAnnotationsTypeDashboard},
				{Action: ac.ActionAnnotationsWrite, Scope: ac.ScopeAnnotationsTypeDashboard},
			},
		},
		Grants: []string{string(models.ROLE_VIEWER)},
	}

	annotationsWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Name:        "fixed:annotations:writer",
			DisplayName: "Annotation writer",
			Description: "Update all annotations.",
			Group:       "Annotations",
			Version:     2,
			Permissions: []ac.Permission{
				{Action: ac.ActionAnnotationsCreate, Scope: ac.ScopeAnnotationsAll},
				{Action: ac.ActionAnnotationsDelete, Scope: ac.ScopeAnnotationsAll},
				{Action: ac.ActionAnnotationsWrite, Scope: ac.ScopeAnnotationsAll},
			},
		},
		Grants: []string{string(models.ROLE_EDITOR)},
	}

	dashboardsCreatorRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     2,
			Name:        "fixed:dashboards:creator",
			DisplayName: "Dashboard creator",
			Description: "Create dashboard in general folder.",
			Group:       "Dashboards",
			Permissions: []ac.Permission{
				{Action: dashboards.ActionFoldersRead, Scope: dashboards.ScopeFoldersProvider.GetResourceScopeUID(ac.GeneralFolderUID)},
				{Action: ac.ActionDashboardsCreate, Scope: dashboards.ScopeFoldersProvider.GetResourceScopeUID(ac.GeneralFolderUID)},
			},
		},
		Grants: []string{"Editor"},
	}

	dashboardsReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:dashboards:reader",
			DisplayName: "Dashboard reader",
			Description: "Read all dashboards.",
			Group:       "Dashboards",
			Permissions: []ac.Permission{
				{Action: ac.ActionDashboardsRead, Scope: ac.ScopeDashboardsAll},
			},
		},
		Grants: []string{"Admin"},
	}

	dashboardsWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:dashboards:writer",
			DisplayName: "Dashboard writer",
			Group:       "Dashboards",
			Description: "Create, read, write or delete all dashboards and their permissions.",
			Permissions: ac.ConcatPermissions(dashboardsReaderRole.Role.Permissions, []ac.Permission{
				{Action: ac.ActionDashboardsWrite, Scope: ac.ScopeDashboardsAll},
				{Action: ac.ActionDashboardsDelete, Scope: ac.ScopeDashboardsAll},
				{Action: ac.ActionDashboardsCreate, Scope: dashboards.ScopeFoldersAll},
				{Action: ac.ActionDashboardsPermissionsRead, Scope: ac.ScopeDashboardsAll},
				{Action: ac.ActionDashboardsPermissionsWrite, Scope: ac.ScopeDashboardsAll},
			}),
		},
		Grants: []string{"Admin"},
	}

	foldersCreatorRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:folders:creator",
			DisplayName: "Folder creator",
			Description: "Create folders.",
			Group:       "Folders",
			Permissions: []ac.Permission{
				{Action: dashboards.ActionFoldersCreate},
			},
		},
		Grants: []string{"Editor"},
	}

	foldersReaderRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:folders:reader",
			DisplayName: "Folder reader",
			Description: "Read all folders and dashboards.",
			Group:       "Folders",
			Permissions: []ac.Permission{
				{Action: dashboards.ActionFoldersRead, Scope: dashboards.ScopeFoldersAll},
				{Action: ac.ActionDashboardsRead, Scope: dashboards.ScopeFoldersAll},
			},
		},
		Grants: []string{"Admin"},
	}

	foldersWriterRole := ac.RoleRegistration{
		Role: ac.RoleDTO{
			Version:     1,
			Name:        "fixed:folders:writer",
			DisplayName: "Folder writer",
			Description: "Create, read, write or delete all folders and dashboards and their permissions.",
			Group:       "Folders",
			Permissions: ac.ConcatPermissions(
				foldersReaderRole.Role.Permissions,
				[]ac.Permission{
					{Action: dashboards.ActionFoldersCreate},
					{Action: dashboards.ActionFoldersWrite, Scope: dashboards.ScopeFoldersAll},
					{Action: dashboards.ActionFoldersDelete, Scope: dashboards.ScopeFoldersAll},
					{Action: ac.ActionDashboardsWrite, Scope: dashboards.ScopeFoldersAll},
					{Action: ac.ActionDashboardsDelete, Scope: dashboards.ScopeFoldersAll},
					{Action: ac.ActionDashboardsCreate, Scope: dashboards.ScopeFoldersAll},
					{Action: ac.ActionDashboardsPermissionsRead, Scope: dashboards.ScopeFoldersAll},
					{Action: ac.ActionDashboardsPermissionsWrite, Scope: dashboards.ScopeFoldersAll},
				}),
		},
		Grants: []string{"Admin"},
	}

	return hs.AccessControl.DeclareFixedRoles(
		provisioningWriterRole, datasourcesReaderRole, datasourcesWriterRole, datasourcesIdReaderRole,
		datasourcesCompatibilityReaderRole, orgReaderRole, orgWriterRole,
		orgMaintainerRole, teamsCreatorRole, teamsWriterRole, datasourcesExplorerRole,
		annotationsReaderRole, dashboardAnnotationsWriterRole, annotationsWriterRole,
		dashboardsCreatorRole, dashboardsReaderRole, dashboardsWriterRole,
		foldersCreatorRole, foldersReaderRole, foldersWriterRole, apikeyReaderRole, apikeyWriterRole,
	)
}

// Evaluators
// here is the list of complex evaluators we use in this package

// orgPreferencesAccessEvaluator is used to protect the "Configure > Preferences" page access
var orgPreferencesAccessEvaluator = ac.EvalAny(
	ac.EvalAll(
		ac.EvalPermission(ActionOrgsRead),
		ac.EvalPermission(ActionOrgsWrite),
	),
	ac.EvalAll(
		ac.EvalPermission(ActionOrgsPreferencesRead),
		ac.EvalPermission(ActionOrgsPreferencesWrite),
	),
)

// orgsAccessEvaluator is used to protect the "Server Admin > Orgs" page access
// (you need to have read access to update or delete orgs; read is the minimum)
var orgsAccessEvaluator = ac.EvalPermission(ActionOrgsRead)

// orgsCreateAccessEvaluator is used to protect the "Server Admin > Orgs > New Org" page access
var orgsCreateAccessEvaluator = ac.EvalAll(
	ac.EvalPermission(ActionOrgsRead),
	ac.EvalPermission(ActionOrgsCreate),
)

// teamsAccessEvaluator is used to protect the "Configuration > Teams" page access
// grants access to a user when they can either create teams or can read and update a team
var teamsAccessEvaluator = ac.EvalAny(
	ac.EvalPermission(ac.ActionTeamsCreate),
	ac.EvalAll(
		ac.EvalPermission(ac.ActionTeamsRead),
		ac.EvalAny(
			ac.EvalPermission(ac.ActionTeamsWrite),
			ac.EvalPermission(ac.ActionTeamsPermissionsWrite),
		),
	),
)

// teamsEditAccessEvaluator is used to protect the "Configuration > Teams > edit" page access
var teamsEditAccessEvaluator = ac.EvalAll(
	ac.EvalPermission(ac.ActionTeamsRead),
	ac.EvalAny(
		ac.EvalPermission(ac.ActionTeamsCreate),
		ac.EvalPermission(ac.ActionTeamsWrite),
		ac.EvalPermission(ac.ActionTeamsPermissionsWrite),
	),
)

// apiKeyAccessEvaluator is used to protect the "Configuration > API keys" page access
var apiKeyAccessEvaluator = ac.EvalPermission(ac.ActionAPIKeyRead)

// serviceAccountAccessEvaluator is used to protect the "Configuration > Service accounts" page access
var serviceAccountAccessEvaluator = ac.EvalPermission(serviceaccounts.ActionRead)

// Metadata helpers
// getAccessControlMetadata returns the accesscontrol metadata associated with a given resource
func (hs *HTTPServer) getAccessControlMetadata(c *models.ReqContext,
	orgID int64, prefix string, resourceID string) ac.Metadata {
	ids := map[string]bool{resourceID: true}
	return hs.getMultiAccessControlMetadata(c, orgID, prefix, ids)[resourceID]
}

// getMultiAccessControlMetadata returns the accesscontrol metadata associated with a given set of resources
// Context must contain permissions in the given org (see LoadPermissionsMiddleware or AuthorizeInOrgMiddleware)
func (hs *HTTPServer) getMultiAccessControlMetadata(c *models.ReqContext,
	orgID int64, prefix string, resourceIDs map[string]bool) map[string]ac.Metadata {
	if hs.AccessControl.IsDisabled() || !c.QueryBool("accesscontrol") {
		return map[string]ac.Metadata{}
	}

	if c.SignedInUser.Permissions == nil {
		return map[string]ac.Metadata{}
	}

	permissions, ok := c.SignedInUser.Permissions[orgID]
	if !ok {
		return map[string]ac.Metadata{}
	}

	return ac.GetResourcesMetadata(c.Req.Context(), permissions, prefix, resourceIDs)
}
