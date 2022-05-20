package accesscontrol

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/infra/localcache"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
)

const (
	ttl            = 30 * time.Second
	cleanInterval  = 2 * time.Minute
	maxPrefixParts = 2
)

func GetResourceScope(resource string, resourceID string) string {
	return Scope(resource, "id", resourceID)
}

func GetResourceScopeUID(resource string, resourceID string) string {
	return Scope(resource, "uid", resourceID)
}

func GetResourceScopeName(resource string, resourceID string) string {
	return Scope(resource, "name", resourceID)
}

func GetResourceScopeType(resource string, typeName string) string {
	return Scope(resource, "type", typeName)
}

func GetResourceAllScope(resource string) string {
	return Scope(resource, "*")
}

func GetResourceAllIDScope(resource string) string {
	return Scope(resource, "id", "*")
}

// Scope builds scope from parts
// e.g. Scope("users", "*") return "users:*"
func Scope(parts ...string) string {
	b := strings.Builder{}
	for i, c := range parts {
		if i != 0 {
			b.WriteRune(':')
		}
		b.WriteString(c)
	}
	return b.String()
}

// Parameter returns injectable scope part, based on URL parameters.
// e.g. Scope("users", Parameter(":id")) or "users:" + Parameter(":id")
func Parameter(key string) string {
	return fmt.Sprintf(`{{ index .URLParams "%s" }}`, key)
}

// Field returns an injectable scope part for selected fields from the request's context available in accesscontrol.ScopeParams.
// e.g. Scope("orgs", Parameter("OrgID")) or "orgs:" + Parameter("OrgID")
func Field(key string) string {
	return fmt.Sprintf(`{{ .%s }}`, key)
}

// ScopeMutator alters a Scope to return a new modified Scope
type ScopeMutator func(context.Context, string) (string, error)

type KeywordScopeResolveFunc func(*models.SignedInUser) (string, error)

// ScopeResolver is used to resolve scope keywords such as `self` or `current` into `id` based scopes and scope attributes such as `name` or `uid` into `id` based scopes.
type ScopeResolver struct {
	keywordResolvers   map[string]KeywordScopeResolveFunc
	attributeResolvers map[string]AttributeScopeResolveFunc
	cache              *localcache.CacheService
	log                log.Logger
}

func NewScopeResolver() ScopeResolver {
	return ScopeResolver{
		keywordResolvers: map[string]KeywordScopeResolveFunc{
			"users:self": resolveUserSelf,
		},
		attributeResolvers: map[string]AttributeScopeResolveFunc{},
		cache:              localcache.New(ttl, cleanInterval),
		log:                log.New("accesscontrol.scoperesolution"),
	}
}

func (s *ScopeResolver) AddKeywordResolver(keyword string, fn KeywordScopeResolveFunc) {
	s.log.Debug("adding keyword resolution for '%v'", keyword)
	s.keywordResolvers[keyword] = fn
}

func (s *ScopeResolver) AddAttributeResolver(prefix string, fn AttributeScopeResolveFunc) {
	s.log.Debug("adding attribute resolution for '%v'", prefix)
	s.attributeResolvers[prefix] = fn
}

func resolveUserSelf(u *models.SignedInUser) (string, error) {
	return Scope("users", "id", fmt.Sprintf("%v", u.UserId)), nil
}

// GetResolveKeywordScopeMutator returns a function to resolve scope with keywords such as `self` or `current` into `id` based scopes
func (s *ScopeResolver) GetResolveKeywordScopeMutator(user *models.SignedInUser) ScopeMutator {
	return func(_ context.Context, scope string) (string, error) {
		var err error
		// By default the scope remains unchanged
		resolvedScope := scope
		if fn, ok := s.keywordResolvers[scope]; ok {
			resolvedScope, err = fn(user)
			if err != nil {
				return "", fmt.Errorf("could not resolve %v: %w", scope, err)
			}
			s.log.Debug("resolved '%v' to '%v'", scope, resolvedScope)
		}
		return resolvedScope, nil
	}
}

type AttributeScopeResolveFunc func(ctx context.Context, orgID int64, initialScope string) (string, error)

// getCacheKey creates an identifier to fetch and store resolution of scopes in the cache
func getCacheKey(orgID int64, scope string) string {
	return fmt.Sprintf("%s-%v", scope, orgID)
}

// GetResolveAttributeScopeMutator returns a function to resolve scopes with attributes such as `name` or `uid` into `id` based scopes
func (s *ScopeResolver) GetResolveAttributeScopeMutator(orgID int64) ScopeMutator {
	return func(ctx context.Context, scope string) (string, error) {
		// Check cache before computing the scope
		if cachedScope, ok := s.cache.Get(getCacheKey(orgID, scope)); ok {
			resolvedScope := cachedScope.(string)
			s.log.Debug("used cache to resolve '%v' to '%v'", scope, resolvedScope)
			return resolvedScope, nil
		}

		var err error
		// By default the scope remains unchanged
		resolvedScope := scope
		prefix := ScopePrefix(scope)
		if fn, ok := s.attributeResolvers[prefix]; ok {
			resolvedScope, err = fn(ctx, orgID, scope)
			if err != nil {
				return "", fmt.Errorf("could not resolve %v: %w", scope, err)
			}
			// Cache result
			s.cache.Set(getCacheKey(orgID, scope), resolvedScope, ttl)
			s.log.Debug("resolved '%v' to '%v'", scope, resolvedScope)
		}
		return resolvedScope, nil
	}
}

// ScopePrefix returns the prefix associated to a given scope
// we assume prefixes are all in the form <resource>:<attribute>:<value>
// ex: "datasources:name:test" returns "datasources:name:"
func ScopePrefix(scope string) string {
	parts := strings.Split(scope, ":")
	// We assume prefixes don't have more than maxPrefixParts parts
	if len(parts) > maxPrefixParts {
		parts = append(parts[:maxPrefixParts], "")
	}
	return strings.Join(parts, ":")
}

//Inject params into the evaluator's templated scopes. e.g. "settings:" + eval.Parameters(":id")
func ScopeInjector(params ScopeParams) ScopeMutator {
	return func(_ context.Context, scope string) (string, error) {
		tmpl, err := template.New("scope").Parse(scope)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err = tmpl.Execute(&buf, params); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}

// ScopeProvider provides methods that construct scopes
type ScopeProvider interface {
	GetResourceScope(resourceID string) string
	GetResourceScopeUID(resourceID string) string
	GetResourceScopeName(resourceID string) string
	GetResourceScopeType(typeName string) string
	GetResourceAllScope() string
	GetResourceAllIDScope() string
}

type scopeProviderImpl struct {
	root string
}

// NewScopeProvider creates a new ScopeProvider that is configured with specific root scope
func NewScopeProvider(root string) ScopeProvider {
	return &scopeProviderImpl{
		root: root,
	}
}

// GetResourceScope returns scope that has the format "<rootScope>:id:<resourceID>"
func (s scopeProviderImpl) GetResourceScope(resourceID string) string {
	return GetResourceScope(s.root, resourceID)
}

// GetResourceScopeUID returns scope that has the format "<rootScope>:uid:<resourceID>"
func (s scopeProviderImpl) GetResourceScopeUID(resourceID string) string {
	return GetResourceScopeUID(s.root, resourceID)
}

// GetResourceScopeName returns scope that has the format "<rootScope>:name:<resourceID>"
func (s scopeProviderImpl) GetResourceScopeName(resourceID string) string {
	return GetResourceScopeName(s.root, resourceID)
}

// GetResourceScopeType returns scope that has the format "<rootScope>:type:<typeName>"
func (s scopeProviderImpl) GetResourceScopeType(typeName string) string {
	return GetResourceScopeType(s.root, typeName)
}

// GetResourceAllScope returns scope that has the format "<rootScope>:*"
func (s scopeProviderImpl) GetResourceAllScope() string {
	return GetResourceAllScope(s.root)
}

// GetResourceAllIDScope returns scope that has the format "<rootScope>:id:*"
func (s scopeProviderImpl) GetResourceAllIDScope() string {
	return GetResourceAllIDScope(s.root)
}
