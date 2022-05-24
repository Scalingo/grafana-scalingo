package plugincontext

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/grafana/grafana/pkg/infra/localcache"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/adapters"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/pluginsettings"
	"github.com/grafana/grafana/pkg/services/secrets"
	"github.com/grafana/grafana/pkg/util/errutil"
)

func ProvideService(cacheService *localcache.CacheService, pluginStore plugins.Store,
	dataSourceCache datasources.CacheService, secretsService secrets.Service,
	pluginSettingsService pluginsettings.Service) *Provider {
	return &Provider{
		cacheService:          cacheService,
		pluginStore:           pluginStore,
		dataSourceCache:       dataSourceCache,
		secretsService:        secretsService,
		pluginSettingsService: pluginSettingsService,
		logger:                log.New("plugincontext"),
	}
}

type Provider struct {
	cacheService          *localcache.CacheService
	pluginStore           plugins.Store
	dataSourceCache       datasources.CacheService
	secretsService        secrets.Service
	pluginSettingsService pluginsettings.Service
	logger                log.Logger
}

// Get allows getting plugin context by its ID. If datasourceUID is not empty string
// then PluginContext.DataSourceInstanceSettings will be resolved and appended to
// returned context.
func (p *Provider) Get(ctx context.Context, pluginID string, datasourceUID string, user *models.SignedInUser, skipCache bool) (backend.PluginContext, bool, error) {
	pc := backend.PluginContext{}
	plugin, exists := p.pluginStore.Plugin(ctx, pluginID)
	if !exists {
		return pc, false, nil
	}

	jsonData := json.RawMessage{}
	decryptedSecureJSONData := map[string]string{}
	var updated time.Time

	ps, err := p.getCachedPluginSettings(ctx, pluginID, user)
	if err != nil {
		// models.ErrPluginSettingNotFound is expected if there's no row found for plugin setting in database (if non-app plugin).
		// If it's not this expected error something is wrong with cache or database and we return the error to the client.
		if !errors.Is(err, models.ErrPluginSettingNotFound) {
			return pc, false, errutil.Wrap("Failed to get plugin settings", err)
		}
	} else {
		jsonData, err = json.Marshal(ps.JSONData)
		if err != nil {
			return pc, false, errutil.Wrap("Failed to unmarshal plugin json data", err)
		}
		decryptedSecureJSONData = p.pluginSettingsService.DecryptedValues(ps)
		updated = ps.Updated
	}

	pCtx := backend.PluginContext{
		OrgID:    user.OrgId,
		PluginID: plugin.ID,
		User:     adapters.BackendUserFromSignedInUser(user),
		AppInstanceSettings: &backend.AppInstanceSettings{
			JSONData:                jsonData,
			DecryptedSecureJSONData: decryptedSecureJSONData,
			Updated:                 updated,
		},
	}

	if datasourceUID != "" {
		ds, err := p.dataSourceCache.GetDatasourceByUID(ctx, datasourceUID, user, skipCache)
		if err != nil {
			return pc, false, errutil.Wrap("Failed to get datasource", err)
		}
		datasourceSettings, err := adapters.ModelToInstanceSettings(ds, p.decryptSecureJsonDataFn())
		if err != nil {
			return pc, false, errutil.Wrap("Failed to convert datasource", err)
		}
		pCtx.DataSourceInstanceSettings = datasourceSettings
	}

	return pCtx, true, nil
}

const pluginSettingsCacheTTL = 5 * time.Second
const pluginSettingsCachePrefix = "plugin-setting-"

func (p *Provider) getCachedPluginSettings(ctx context.Context, pluginID string, user *models.SignedInUser) (*pluginsettings.DTO, error) {
	cacheKey := pluginSettingsCachePrefix + pluginID

	if cached, found := p.cacheService.Get(cacheKey); found {
		ps := cached.(*pluginsettings.DTO)
		if ps.OrgID == user.OrgId {
			return ps, nil
		}
	}

	ps, err := p.pluginSettingsService.GetPluginSettingByPluginID(ctx, &pluginsettings.GetByPluginIDArgs{
		PluginID: pluginID,
		OrgID:    user.OrgId,
	})
	if err != nil {
		return nil, err
	}

	p.cacheService.Set(cacheKey, ps, pluginSettingsCacheTTL)
	return ps, nil
}

func (p *Provider) decryptSecureJsonDataFn() func(map[string][]byte) map[string]string {
	return func(m map[string][]byte) map[string]string {
		decryptedJsonData, err := p.secretsService.DecryptJsonData(context.Background(), m)
		if err != nil {
			p.logger.Error("Failed to decrypt secure json data", "error", err)
		}
		return decryptedJsonData
	}
}
