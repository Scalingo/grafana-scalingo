package acimpl

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/localcache"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/metrics"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/api"
	"github.com/grafana/grafana/pkg/services/accesscontrol/database"
	"github.com/grafana/grafana/pkg/services/accesscontrol/pluginutils"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
)

var _ plugins.RoleRegistry = &Service{}

const (
	cacheTTL = 10 * time.Second
)

func ProvideService(cfg *setting.Cfg, store db.DB, routeRegister routing.RouteRegister, cache *localcache.CacheService,
	features *featuremgmt.FeatureManager) (*Service, error) {
	service := ProvideOSSService(cfg, database.ProvideService(store), cache, features)

	if !accesscontrol.IsDisabled(cfg) {
		api.NewAccessControlAPI(routeRegister, service).RegisterAPIEndpoints()
		if err := accesscontrol.DeclareFixedRoles(service); err != nil {
			return nil, err
		}
	}

	return service, nil
}

func ProvideOSSService(cfg *setting.Cfg, store store, cache *localcache.CacheService, features *featuremgmt.FeatureManager) *Service {
	s := &Service{
		cfg:      cfg,
		store:    store,
		log:      log.New("accesscontrol.service"),
		cache:    cache,
		roles:    accesscontrol.BuildBasicRoleDefinitions(),
		features: features,
	}

	return s
}

type store interface {
	GetUserPermissions(ctx context.Context, query accesscontrol.GetUserPermissionsQuery) ([]accesscontrol.Permission, error)
	DeleteUserPermissions(ctx context.Context, orgID, userID int64) error
}

// Service is the service implementing role based access control.
type Service struct {
	log           log.Logger
	cfg           *setting.Cfg
	store         store
	cache         *localcache.CacheService
	registrations accesscontrol.RegistrationList
	roles         map[string]*accesscontrol.RoleDTO
	features      *featuremgmt.FeatureManager
}

func (s *Service) GetUsageStats(_ context.Context) map[string]interface{} {
	enabled := 0
	if !accesscontrol.IsDisabled(s.cfg) {
		enabled = 1
	}

	return map[string]interface{}{
		"stats.oss.accesscontrol.enabled.count": enabled,
	}
}

// GetUserPermissions returns user permissions based on built-in roles
func (s *Service) GetUserPermissions(ctx context.Context, user *user.SignedInUser, options accesscontrol.Options) ([]accesscontrol.Permission, error) {
	timer := prometheus.NewTimer(metrics.MAccessPermissionsSummary)
	defer timer.ObserveDuration()

	if !s.cfg.RBACPermissionCache || !user.HasUniqueId() {
		return s.getUserPermissions(ctx, user, options)
	}

	return s.getCachedUserPermissions(ctx, user, options)
}

func (s *Service) getUserPermissions(ctx context.Context, user *user.SignedInUser, options accesscontrol.Options) ([]accesscontrol.Permission, error) {
	permissions := make([]accesscontrol.Permission, 0)
	for _, builtin := range accesscontrol.GetOrgRoles(user) {
		if basicRole, ok := s.roles[builtin]; ok {
			permissions = append(permissions, basicRole.Permissions...)
		}
	}

	dbPermissions, err := s.store.GetUserPermissions(ctx, accesscontrol.GetUserPermissionsQuery{
		OrgID:      user.OrgID,
		UserID:     user.UserID,
		Roles:      accesscontrol.GetOrgRoles(user),
		TeamIDs:    user.Teams,
		RolePrefix: accesscontrol.ManagedRolePrefix,
	})
	if err != nil {
		return nil, err
	}

	return append(permissions, dbPermissions...), nil
}

func (s *Service) getCachedUserPermissions(ctx context.Context, user *user.SignedInUser, options accesscontrol.Options) ([]accesscontrol.Permission, error) {
	key, err := permissionCacheKey(user)
	if err != nil {
		return nil, err
	}

	if !options.ReloadCache {
		permissions, ok := s.cache.Get(key)
		if ok {
			s.log.Debug("using cached permissions", "key", key)
			return permissions.([]accesscontrol.Permission), nil
		}
	}

	s.log.Debug("fetch permissions from store", "key", key)
	permissions, err := s.getUserPermissions(ctx, user, options)
	if err != nil {
		return nil, err
	}

	s.log.Debug("cache permissions", "key", key)
	s.cache.Set(key, permissions, cacheTTL)

	return permissions, nil
}

func (s *Service) ClearUserPermissionCache(user *user.SignedInUser) {
	key, err := permissionCacheKey(user)
	if err != nil {
		return
	}
	s.cache.Delete(key)
}

func (s *Service) DeleteUserPermissions(ctx context.Context, orgID int64, userID int64) error {
	return s.store.DeleteUserPermissions(ctx, orgID, userID)
}

// DeclareFixedRoles allow the caller to declare, to the service, fixed roles and their assignments
// to organization roles ("Viewer", "Editor", "Admin") or "Grafana Admin"
func (s *Service) DeclareFixedRoles(registrations ...accesscontrol.RoleRegistration) error {
	// If accesscontrol is disabled no need to register roles
	if accesscontrol.IsDisabled(s.cfg) {
		return nil
	}

	for _, r := range registrations {
		err := accesscontrol.ValidateFixedRole(r.Role)
		if err != nil {
			return err
		}

		err = accesscontrol.ValidateBuiltInRoles(r.Grants)
		if err != nil {
			return err
		}

		s.registrations.Append(r)
	}

	return nil
}

// RegisterFixedRoles registers all declared roles in RAM
func (s *Service) RegisterFixedRoles(ctx context.Context) error {
	// If accesscontrol is disabled no need to register roles
	if accesscontrol.IsDisabled(s.cfg) {
		return nil
	}
	s.registrations.Range(func(registration accesscontrol.RoleRegistration) bool {
		for br := range accesscontrol.BuiltInRolesWithParents(registration.Grants) {
			if basicRole, ok := s.roles[br]; ok {
				basicRole.Permissions = append(basicRole.Permissions, registration.Role.Permissions...)
			} else {
				s.log.Error("Unknown builtin role", "builtInRole", br)
			}
		}
		return true
	})
	return nil
}

func (s *Service) IsDisabled() bool {
	return accesscontrol.IsDisabled(s.cfg)
}

func permissionCacheKey(user *user.SignedInUser) (string, error) {
	key, err := user.GetCacheKey()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("rbac-permissions-%s", key), nil
}

// DeclarePluginRoles allow the caller to declare, to the service, plugin roles and their assignments
// to organization roles ("Viewer", "Editor", "Admin") or "Grafana Admin"
func (s *Service) DeclarePluginRoles(_ context.Context, ID, name string, regs []plugins.RoleRegistration) error {
	// If accesscontrol is disabled no need to register roles
	if accesscontrol.IsDisabled(s.cfg) {
		return nil
	}

	// Protect behind feature toggle
	if !s.features.IsEnabled(featuremgmt.FlagAccessControlOnCall) {
		return nil
	}

	acRegs := pluginutils.ToRegistrations(ID, name, regs)
	for _, r := range acRegs {
		if err := pluginutils.ValidatePluginRole(ID, r.Role); err != nil {
			return err
		}

		if err := accesscontrol.ValidateBuiltInRoles(r.Grants); err != nil {
			return err
		}

		s.log.Debug("Registering plugin role", "role", r.Role.Name)
		s.registrations.Append(r)
	}

	return nil
}
