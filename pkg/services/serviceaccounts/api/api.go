package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/serviceaccounts"
	"github.com/grafana/grafana/pkg/services/serviceaccounts/database"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

type ServiceAccountsAPI struct {
	cfg                  *setting.Cfg
	service              serviceaccounts.Service
	accesscontrol        accesscontrol.AccessControl
	accesscontrolService accesscontrol.Service
	RouterRegister       routing.RouteRegister
	store                serviceaccounts.Store
	log                  log.Logger
	permissionService    accesscontrol.ServiceAccountPermissionsService
}

func NewServiceAccountsAPI(
	cfg *setting.Cfg,
	service serviceaccounts.Service,
	accesscontrol accesscontrol.AccessControl,
	accesscontrolService accesscontrol.Service,
	routerRegister routing.RouteRegister,
	store serviceaccounts.Store,
	permissionService accesscontrol.ServiceAccountPermissionsService,
) *ServiceAccountsAPI {
	return &ServiceAccountsAPI{
		cfg:                  cfg,
		service:              service,
		accesscontrol:        accesscontrol,
		accesscontrolService: accesscontrolService,
		RouterRegister:       routerRegister,
		store:                store,
		log:                  log.New("serviceaccounts.api"),
		permissionService:    permissionService,
	}
}

func (api *ServiceAccountsAPI) RegisterAPIEndpoints() {
	auth := accesscontrol.Middleware(api.accesscontrol)
	api.RouterRegister.Group("/api/serviceaccounts", func(serviceAccountsRoute routing.RouteRegister) {
		serviceAccountsRoute.Get("/search", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionRead)), routing.Wrap(api.SearchOrgServiceAccountsWithPaging))
		serviceAccountsRoute.Post("/", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionCreate)), routing.Wrap(api.CreateServiceAccount))
		serviceAccountsRoute.Get("/:serviceAccountId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionRead, serviceaccounts.ScopeID)), routing.Wrap(api.RetrieveServiceAccount))
		serviceAccountsRoute.Patch("/:serviceAccountId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionWrite, serviceaccounts.ScopeID)), routing.Wrap(api.UpdateServiceAccount))
		serviceAccountsRoute.Delete("/:serviceAccountId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionDelete, serviceaccounts.ScopeID)), routing.Wrap(api.DeleteServiceAccount))
		serviceAccountsRoute.Get("/:serviceAccountId/tokens", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionRead, serviceaccounts.ScopeID)), routing.Wrap(api.ListTokens))
		serviceAccountsRoute.Post("/:serviceAccountId/tokens", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionWrite, serviceaccounts.ScopeID)), routing.Wrap(api.CreateToken))
		serviceAccountsRoute.Delete("/:serviceAccountId/tokens/:tokenId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionWrite, serviceaccounts.ScopeID)), routing.Wrap(api.DeleteToken))
		serviceAccountsRoute.Get("/migrationstatus", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionRead)), routing.Wrap(api.GetAPIKeysMigrationStatus))
		serviceAccountsRoute.Post("/hideApiKeys", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionCreate)), routing.Wrap(api.HideApiKeysTab))
		serviceAccountsRoute.Post("/migrate", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionCreate)), routing.Wrap(api.MigrateApiKeysToServiceAccounts))
		serviceAccountsRoute.Post("/migrate/:keyId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionCreate)), routing.Wrap(api.ConvertToServiceAccount))
		serviceAccountsRoute.Post("/:serviceAccountId/revert/:keyId", auth(middleware.ReqOrgAdmin,
			accesscontrol.EvalPermission(serviceaccounts.ActionDelete, serviceaccounts.ScopeID)), routing.Wrap(api.RevertApiKey))
	})
}

// swagger:route POST /serviceaccounts service_accounts createServiceAccount
//
// # Create service account
//
// Required permissions (See note in the [introduction](https://grafana.com/docs/grafana/latest/developers/http_api/serviceaccount/#service-account-api) for an explanation):
// action: `serviceaccounts:write` scope: `serviceaccounts:*`
//
// Requires basic authentication and that the authenticated user is a Grafana Admin.
//
// Responses:
// 201: createServiceAccountResponse
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 500: internalServerError
func (api *ServiceAccountsAPI) CreateServiceAccount(c *models.ReqContext) response.Response {
	cmd := serviceaccounts.CreateServiceAccountForm{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "Bad request data", err)
	}

	if err := api.validateRole(cmd.Role, &c.OrgRole); err != nil {
		switch {
		case errors.Is(err, serviceaccounts.ErrServiceAccountInvalidRole):
			return response.Error(http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, serviceaccounts.ErrServiceAccountRolePrivilegeDenied):
			return response.Error(http.StatusForbidden, err.Error(), err)
		default:
			return response.Error(http.StatusInternalServerError, "failed to create service account", err)
		}
	}

	serviceAccount, err := api.store.CreateServiceAccount(c.Req.Context(), c.OrgID, &cmd)
	switch {
	case errors.Is(err, database.ErrServiceAccountAlreadyExists):
		return response.Error(http.StatusBadRequest, "Failed to create service account", err)
	case err != nil:
		return response.Error(http.StatusInternalServerError, "Failed to create service account", err)
	}

	if !api.accesscontrol.IsDisabled() {
		if c.SignedInUser.IsRealUser() {
			if _, err := api.permissionService.SetUserPermission(c.Req.Context(), c.OrgID, accesscontrol.User{ID: c.SignedInUser.UserID}, strconv.FormatInt(serviceAccount.Id, 10), "Admin"); err != nil {
				return response.Error(http.StatusInternalServerError, "Failed to set permissions for service account creator", err)
			}
		}

		// Clear permission cache for the user who's created the service account, so that new permissions are fetched for their next call
		// Required for cases when caller wants to immediately interact with the newly created object
		api.accesscontrolService.ClearUserPermissionCache(c.SignedInUser)
	}

	return response.JSON(http.StatusCreated, serviceAccount)
}

// swagger:route GET /serviceaccounts/{serviceAccountId} service_accounts retrieveServiceAccount
//
// # Get single serviceaccount by Id
//
// Required permissions (See note in the [introduction](https://grafana.com/docs/grafana/latest/developers/http_api/serviceaccount/#service-account-api) for an explanation):
// action: `serviceaccounts:read` scope: `serviceaccounts:id:1` (single service account)
//
// Responses:
// 200: retrieveServiceAccountResponse
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (api *ServiceAccountsAPI) RetrieveServiceAccount(ctx *models.ReqContext) response.Response {
	scopeID, err := strconv.ParseInt(web.Params(ctx.Req)[":serviceAccountId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "Service Account ID is invalid", err)
	}

	serviceAccount, err := api.store.RetrieveServiceAccount(ctx.Req.Context(), ctx.OrgID, scopeID)
	if err != nil {
		switch {
		case errors.Is(err, serviceaccounts.ErrServiceAccountNotFound):
			return response.Error(http.StatusNotFound, "Failed to retrieve service account", err)
		default:
			return response.Error(http.StatusInternalServerError, "Failed to retrieve service account", err)
		}
	}

	saIDString := strconv.FormatInt(serviceAccount.Id, 10)
	metadata := api.getAccessControlMetadata(ctx, map[string]bool{saIDString: true})
	serviceAccount.AvatarUrl = dtos.GetGravatarUrlWithDefault("", serviceAccount.Name)
	serviceAccount.AccessControl = metadata[saIDString]

	tokens, err := api.store.ListTokens(ctx.Req.Context(), &serviceaccounts.GetSATokensQuery{
		OrgID:            &serviceAccount.OrgId,
		ServiceAccountID: &serviceAccount.Id,
	})
	if err != nil {
		api.log.Warn("Failed to list tokens for service account", "serviceAccount", serviceAccount.Id)
	}
	serviceAccount.Tokens = int64(len(tokens))

	return response.JSON(http.StatusOK, serviceAccount)
}

// swagger:route PATCH /serviceaccounts/{serviceAccountId} service_accounts updateServiceAccount
//
// # Update service account
//
// Required permissions (See note in the [introduction](https://grafana.com/docs/grafana/latest/developers/http_api/serviceaccount/#service-account-api) for an explanation):
// action: `serviceaccounts:write` scope: `serviceaccounts:id:1` (single service account)
//
// Responses:
// 200: updateServiceAccountResponse
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 404: notFoundError
// 500: internalServerError
func (api *ServiceAccountsAPI) UpdateServiceAccount(c *models.ReqContext) response.Response {
	scopeID, err := strconv.ParseInt(web.Params(c.Req)[":serviceAccountId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "Service Account ID is invalid", err)
	}

	cmd := serviceaccounts.UpdateServiceAccountForm{}
	if err := web.Bind(c.Req, &cmd); err != nil {
		return response.Error(http.StatusBadRequest, "Bad request data", err)
	}

	if err := api.validateRole(cmd.Role, &c.OrgRole); err != nil {
		switch {
		case errors.Is(err, serviceaccounts.ErrServiceAccountInvalidRole):
			return response.Error(http.StatusBadRequest, err.Error(), err)
		case errors.Is(err, serviceaccounts.ErrServiceAccountRolePrivilegeDenied):
			return response.Error(http.StatusForbidden, err.Error(), err)
		default:
			return response.Error(http.StatusInternalServerError, "failed to update service account", err)
		}
	}

	resp, err := api.store.UpdateServiceAccount(c.Req.Context(), c.OrgID, scopeID, &cmd)
	if err != nil {
		switch {
		case errors.Is(err, serviceaccounts.ErrServiceAccountNotFound):
			return response.Error(http.StatusNotFound, "Failed to retrieve service account", err)
		default:
			return response.Error(http.StatusInternalServerError, "Failed update service account", err)
		}
	}

	saIDString := strconv.FormatInt(resp.Id, 10)
	metadata := api.getAccessControlMetadata(c, map[string]bool{saIDString: true})
	resp.AvatarUrl = dtos.GetGravatarUrlWithDefault("", resp.Name)
	resp.AccessControl = metadata[saIDString]

	return response.JSON(http.StatusOK, util.DynMap{
		"message":        "Service account updated",
		"id":             resp.Id,
		"name":           resp.Name,
		"serviceaccount": resp,
	})
}

func (api *ServiceAccountsAPI) validateRole(r *org.RoleType, orgRole *org.RoleType) error {
	if r != nil && !r.IsValid() {
		return serviceaccounts.ErrServiceAccountInvalidRole
	}
	if r != nil && !orgRole.Includes(*r) {
		return serviceaccounts.ErrServiceAccountRolePrivilegeDenied
	}
	return nil
}

// swagger:route DELETE /serviceaccounts/{serviceAccountId} service_accounts deleteServiceAccount
//
// # Delete service account
//
// Required permissions (See note in the [introduction](https://grafana.com/docs/grafana/latest/developers/http_api/serviceaccount/#service-account-api) for an explanation):
// action: `serviceaccounts:delete` scope: `serviceaccounts:id:1` (single service account)
//
// Responses:
// 200: okResponse
// 400: badRequestError
// 401: unauthorisedError
// 403: forbiddenError
// 500: internalServerError
func (api *ServiceAccountsAPI) DeleteServiceAccount(ctx *models.ReqContext) response.Response {
	scopeID, err := strconv.ParseInt(web.Params(ctx.Req)[":serviceAccountId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "Service account ID is invalid", err)
	}
	err = api.service.DeleteServiceAccount(ctx.Req.Context(), ctx.OrgID, scopeID)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Service account deletion error", err)
	}
	return response.Success("Service account deleted")
}

// swagger:route GET /serviceaccounts/search service_accounts searchOrgServiceAccountsWithPaging
//
// # Search service accounts with paging
//
// Required permissions (See note in the [introduction](https://grafana.com/docs/grafana/latest/developers/http_api/serviceaccount/#service-account-api) for an explanation):
// action: `serviceaccounts:read` scope: `serviceaccounts:*`
//
// Responses:
// 200: searchOrgServiceAccountsWithPagingResponse
// 401: unauthorisedError
// 403: forbiddenError
// 500: internalServerError
func (api *ServiceAccountsAPI) SearchOrgServiceAccountsWithPaging(c *models.ReqContext) response.Response {
	ctx := c.Req.Context()
	perPage := c.QueryInt("perpage")
	if perPage <= 0 {
		perPage = 1000
	}
	page := c.QueryInt("page")
	if page < 1 {
		page = 1
	}
	// its okay that it fails, it is only filtering that might be weird, but to safe quard against any weird incoming query param
	onlyWithExpiredTokens := c.QueryBool("expiredTokens")
	onlyDisabled := c.QueryBool("disabled")
	filter := serviceaccounts.FilterIncludeAll
	if onlyWithExpiredTokens {
		filter = serviceaccounts.FilterOnlyExpiredTokens
	}
	if onlyDisabled {
		filter = serviceaccounts.FilterOnlyDisabled
	}
	serviceAccountSearch, err := api.store.SearchOrgServiceAccounts(ctx, c.OrgID, c.Query("query"), filter, page, perPage, c.SignedInUser)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Failed to get service accounts for current organization", err)
	}

	saIDs := map[string]bool{}
	for i := range serviceAccountSearch.ServiceAccounts {
		sa := serviceAccountSearch.ServiceAccounts[i]
		sa.AvatarUrl = dtos.GetGravatarUrlWithDefault("", sa.Name)

		saIDString := strconv.FormatInt(sa.Id, 10)
		saIDs[saIDString] = true
		metadata := api.getAccessControlMetadata(c, map[string]bool{saIDString: true})
		sa.AccessControl = metadata[strconv.FormatInt(sa.Id, 10)]
		tokens, err := api.store.ListTokens(ctx, &serviceaccounts.GetSATokensQuery{
			OrgID: &sa.OrgId, ServiceAccountID: &sa.Id,
		})
		if err != nil {
			api.log.Warn("Failed to list tokens for service account", "serviceAccount", sa.Id)
		}
		sa.Tokens = int64(len(tokens))
	}

	return response.JSON(http.StatusOK, serviceAccountSearch)
}

// GET /api/serviceaccounts/migrationstatus
func (api *ServiceAccountsAPI) GetAPIKeysMigrationStatus(ctx *models.ReqContext) response.Response {
	upgradeStatus, err := api.store.GetAPIKeysMigrationStatus(ctx.Req.Context(), ctx.OrgID)
	if err != nil {
		return response.Error(http.StatusInternalServerError, "Internal server error", err)
	}
	return response.JSON(http.StatusOK, upgradeStatus)
}

// POST /api/serviceaccounts/hideapikeys
func (api *ServiceAccountsAPI) HideApiKeysTab(ctx *models.ReqContext) response.Response {
	if err := api.store.HideApiKeysTab(ctx.Req.Context(), ctx.OrgID); err != nil {
		return response.Error(http.StatusInternalServerError, "Internal server error", err)
	}
	return response.Success("API keys hidden")
}

// POST /api/serviceaccounts/migrate
func (api *ServiceAccountsAPI) MigrateApiKeysToServiceAccounts(ctx *models.ReqContext) response.Response {
	if err := api.store.MigrateApiKeysToServiceAccounts(ctx.Req.Context(), ctx.OrgID); err != nil {
		return response.Error(http.StatusInternalServerError, "Internal server error", err)
	}

	return response.Success("API keys migrated to service accounts")
}

// POST /api/serviceaccounts/migrate/:keyId
func (api *ServiceAccountsAPI) ConvertToServiceAccount(ctx *models.ReqContext) response.Response {
	keyId, err := strconv.ParseInt(web.Params(ctx.Req)[":keyId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "Key ID is invalid", err)
	}

	if err := api.store.MigrateApiKey(ctx.Req.Context(), ctx.OrgID, keyId); err != nil {
		return response.Error(http.StatusInternalServerError, "Error converting API key", err)
	}

	return response.Success("Service accounts migrated")
}

// POST /api/serviceaccounts/revert/:keyId
func (api *ServiceAccountsAPI) RevertApiKey(ctx *models.ReqContext) response.Response {
	keyId, err := strconv.ParseInt(web.Params(ctx.Req)[":keyId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "key ID is invalid", err)
	}
	serviceAccountId, err := strconv.ParseInt(web.Params(ctx.Req)[":serviceAccountId"], 10, 64)
	if err != nil {
		return response.Error(http.StatusBadRequest, "service account ID is invalid", err)
	}

	if err := api.store.RevertApiKey(ctx.Req.Context(), serviceAccountId, keyId); err != nil {
		return response.Error(http.StatusInternalServerError, "error reverting to API key", err)
	}
	return response.Success("reverted service account to API key")
}

func (api *ServiceAccountsAPI) getAccessControlMetadata(c *models.ReqContext, saIDs map[string]bool) map[string]accesscontrol.Metadata {
	if api.accesscontrol.IsDisabled() || !c.QueryBool("accesscontrol") {
		return map[string]accesscontrol.Metadata{}
	}

	if c.SignedInUser.Permissions == nil {
		return map[string]accesscontrol.Metadata{}
	}

	permissions, ok := c.SignedInUser.Permissions[c.OrgID]
	if !ok {
		return map[string]accesscontrol.Metadata{}
	}

	return accesscontrol.GetResourcesMetadata(c.Req.Context(), permissions, "serviceaccounts:id:", saIDs)
}

// swagger:parameters searchOrgServiceAccountsWithPaging
type SearchOrgServiceAccountsWithPagingParams struct {
	// in:query
	// required:false
	Disabled bool `jsson:"disabled"`
	// in:query
	// required:false
	ExpiredTokens bool `json:"expiredTokens"`
	// It will return results where the query value is contained in one of the name.
	// Query values with spaces need to be URL encoded.
	// in:query
	// required:false
	Query string `json:"query"`
	// The default value is 1000.
	// in:query
	// required:false
	PerPage int `json:"perpage"`
	// The default value is 1.
	// in:query
	// required:false
	Page int `json:"page"`
}

// swagger:parameters createServiceAccount
type CreateServiceAccountParams struct {
	//in:body
	Body serviceaccounts.CreateServiceAccountForm
}

// swagger:parameters retrieveServiceAccount
type RetrieveServiceAccountParams struct {
	// in:path
	ServiceAccountId int64 `json:"serviceAccountId"`
}

// swagger:parameters updateServiceAccount
type UpdateServiceAccountParams struct {
	// in:path
	ServiceAccountId int64 `json:"serviceAccountId"`
	// in:body
	Body serviceaccounts.UpdateServiceAccountForm
}

// swagger:parameters deleteServiceAccount
type DeleteServiceAccountParams struct {
	// in:path
	ServiceAccountId int64 `json:"serviceAccountId"`
}

// swagger:response searchOrgServiceAccountsWithPagingResponse
type SearchOrgServiceAccountsWithPagingResponse struct {
	// in:body
	Body *serviceaccounts.SearchServiceAccountsResult
}

// swagger:response createServiceAccountResponse
type CreateServiceAccountResponse struct {
	// in:body
	Body *serviceaccounts.ServiceAccountDTO
}

// swagger:response retrieveServiceAccountResponse
type RetrieveServiceAccountResponse struct {
	// in:body
	Body *serviceaccounts.ServiceAccountDTO
}

// swagger:response updateServiceAccountResponse
type UpdateServiceAccountResponse struct {
	// in:body
	Body struct {
		Message        string                                    `json:"message"`
		ID             int64                                     `json:"id"`
		Name           string                                    `json:"name"`
		ServiceAccount *serviceaccounts.ServiceAccountProfileDTO `json:"serviceaccount"`
	}
}
