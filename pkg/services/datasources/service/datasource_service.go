package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-azure-sdk-go/azcredentials"
	"github.com/grafana/grafana-azure-sdk-go/azhttpclient"
	sdkhttpclient "github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/httpclient"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/secrets"
	"github.com/grafana/grafana/pkg/services/secrets/kvstore"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/prometheus/buffered/promclient"
)

type Service struct {
	SQLStore           *sqlstore.SQLStore
	SecretsStore       kvstore.SecretsKVStore
	SecretsService     secrets.Service
	cfg                *setting.Cfg
	features           featuremgmt.FeatureToggles
	permissionsService accesscontrol.DatasourcePermissionsService
	ac                 accesscontrol.AccessControl

	ptc proxyTransportCache
}

type proxyTransportCache struct {
	cache map[int64]cachedRoundTripper
	sync.Mutex
}

type cachedRoundTripper struct {
	updated      time.Time
	roundTripper http.RoundTripper
}

func ProvideService(
	store *sqlstore.SQLStore, secretsService secrets.Service, secretsStore kvstore.SecretsKVStore, cfg *setting.Cfg,
	features featuremgmt.FeatureToggles, ac accesscontrol.AccessControl, datasourcePermissionsService accesscontrol.DatasourcePermissionsService,
) *Service {
	s := &Service{
		SQLStore:       store,
		SecretsStore:   secretsStore,
		SecretsService: secretsService,
		ptc: proxyTransportCache{
			cache: make(map[int64]cachedRoundTripper),
		},
		cfg:                cfg,
		features:           features,
		permissionsService: datasourcePermissionsService,
		ac:                 ac,
	}

	ac.RegisterScopeAttributeResolver(NewNameScopeResolver(store))
	ac.RegisterScopeAttributeResolver(NewIDScopeResolver(store))

	return s
}

// DataSourceRetriever interface for retrieving a datasource.
type DataSourceRetriever interface {
	// GetDataSource gets a datasource.
	GetDataSource(ctx context.Context, query *models.GetDataSourceQuery) error
}

const secretType = "datasource"

// NewNameScopeResolver provides an ScopeAttributeResolver able to
// translate a scope prefixed with "datasources:name:" into an uid based scope.
func NewNameScopeResolver(db DataSourceRetriever) (string, accesscontrol.ScopeAttributeResolver) {
	prefix := datasources.ScopeProvider.GetResourceScopeName("")
	return prefix, accesscontrol.ScopeAttributeResolverFunc(func(ctx context.Context, orgID int64, initialScope string) ([]string, error) {
		if !strings.HasPrefix(initialScope, prefix) {
			return nil, accesscontrol.ErrInvalidScope
		}

		dsName := initialScope[len(prefix):]
		if dsName == "" {
			return nil, accesscontrol.ErrInvalidScope
		}

		query := models.GetDataSourceQuery{Name: dsName, OrgId: orgID}
		if err := db.GetDataSource(ctx, &query); err != nil {
			return nil, err
		}

		return []string{datasources.ScopeProvider.GetResourceScopeUID(query.Result.Uid)}, nil
	})
}

// NewIDScopeResolver provides an ScopeAttributeResolver able to
// translate a scope prefixed with "datasources:id:" into an uid based scope.
func NewIDScopeResolver(db DataSourceRetriever) (string, accesscontrol.ScopeAttributeResolver) {
	prefix := datasources.ScopeProvider.GetResourceScope("")
	return prefix, accesscontrol.ScopeAttributeResolverFunc(func(ctx context.Context, orgID int64, initialScope string) ([]string, error) {
		if !strings.HasPrefix(initialScope, prefix) {
			return nil, accesscontrol.ErrInvalidScope
		}

		id := initialScope[len(prefix):]
		if id == "" {
			return nil, accesscontrol.ErrInvalidScope
		}

		dsID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, accesscontrol.ErrInvalidScope
		}

		query := models.GetDataSourceQuery{Id: dsID, OrgId: orgID}
		if err := db.GetDataSource(ctx, &query); err != nil {
			return nil, err
		}

		return []string{datasources.ScopeProvider.GetResourceScopeUID(query.Result.Uid)}, nil
	})
}

func (s *Service) GetDataSource(ctx context.Context, query *models.GetDataSourceQuery) error {
	return s.SQLStore.GetDataSource(ctx, query)
}

func (s *Service) GetDataSources(ctx context.Context, query *models.GetDataSourcesQuery) error {
	return s.SQLStore.GetDataSources(ctx, query)
}

func (s *Service) GetDataSourcesByType(ctx context.Context, query *models.GetDataSourcesByTypeQuery) error {
	return s.SQLStore.GetDataSourcesByType(ctx, query)
}

func (s *Service) AddDataSource(ctx context.Context, cmd *models.AddDataSourceCommand) error {
	var err error
	// this is here for backwards compatibility
	cmd.EncryptedSecureJsonData, err = s.SecretsService.EncryptJsonData(ctx, cmd.SecureJsonData, secrets.WithoutScope())
	if err != nil {
		return err
	}

	if err := s.SQLStore.AddDataSource(ctx, cmd); err != nil {
		return err
	}

	secret, err := json.Marshal(cmd.SecureJsonData)
	if err != nil {
		return err
	}

	err = s.SecretsStore.Set(ctx, cmd.OrgId, cmd.Name, secretType, string(secret))
	if err != nil {
		return err
	}

	if !s.ac.IsDisabled() {
		// This belongs in Data source permissions, and we probably want
		// to do this with a hook in the store and rollback on fail.
		// We can't use events, because there's no way to communicate
		// failure, and we want "not being able to set default perms"
		// to fail the creation.
		permissions := []accesscontrol.SetResourcePermissionCommand{
			{BuiltinRole: "Viewer", Permission: "Query"},
			{BuiltinRole: "Editor", Permission: "Query"},
		}
		if cmd.UserId != 0 {
			permissions = append(permissions, accesscontrol.SetResourcePermissionCommand{UserID: cmd.UserId, Permission: "Edit"})
		}
		if _, err := s.permissionsService.SetPermissions(ctx, cmd.OrgId, cmd.Result.Uid, permissions...); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) DeleteDataSource(ctx context.Context, cmd *models.DeleteDataSourceCommand) error {
	err := s.SQLStore.DeleteDataSource(ctx, cmd)
	if err != nil {
		return err
	}
	return s.SecretsStore.Del(ctx, cmd.OrgID, cmd.Name, secretType)
}

func (s *Service) UpdateDataSource(ctx context.Context, cmd *models.UpdateDataSourceCommand) error {
	var err error

	query := &models.GetDataSourceQuery{
		Id:    cmd.Id,
		OrgId: cmd.OrgId,
	}
	err = s.SQLStore.GetDataSource(ctx, query)
	if err != nil {
		return err
	}

	err = s.fillWithSecureJSONData(ctx, cmd, query.Result)
	if err != nil {
		return err
	}

	err = s.SQLStore.UpdateDataSource(ctx, cmd)
	if err != nil {
		return err
	}

	if query.Result.Name != cmd.Name {
		err = s.SecretsStore.Rename(ctx, cmd.OrgId, query.Result.Name, secretType, cmd.Name)
		if err != nil {
			return err
		}
	}

	secret, err := json.Marshal(cmd.SecureJsonData)
	if err != nil {
		return err
	}

	return s.SecretsStore.Set(ctx, cmd.OrgId, cmd.Name, secretType, string(secret))
}

func (s *Service) GetDefaultDataSource(ctx context.Context, query *models.GetDefaultDataSourceQuery) error {
	return s.SQLStore.GetDefaultDataSource(ctx, query)
}

func (s *Service) GetHTTPClient(ctx context.Context, ds *models.DataSource, provider httpclient.Provider) (*http.Client, error) {
	transport, err := s.GetHTTPTransport(ctx, ds, provider)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout:   s.getTimeout(ds),
		Transport: transport,
	}, nil
}

func (s *Service) GetHTTPTransport(ctx context.Context, ds *models.DataSource, provider httpclient.Provider,
	customMiddlewares ...sdkhttpclient.Middleware) (http.RoundTripper, error) {
	s.ptc.Lock()
	defer s.ptc.Unlock()

	if t, present := s.ptc.cache[ds.Id]; present && ds.Updated.Equal(t.updated) {
		return t.roundTripper, nil
	}

	opts, err := s.httpClientOptions(ctx, ds)
	if err != nil {
		return nil, err
	}

	opts.Middlewares = append(opts.Middlewares, customMiddlewares...)

	rt, err := provider.GetTransport(*opts)
	if err != nil {
		return nil, err
	}

	s.ptc.cache[ds.Id] = cachedRoundTripper{
		roundTripper: rt,
		updated:      ds.Updated,
	}

	return rt, nil
}

func (s *Service) GetTLSConfig(ctx context.Context, ds *models.DataSource, httpClientProvider httpclient.Provider) (*tls.Config, error) {
	opts, err := s.httpClientOptions(ctx, ds)
	if err != nil {
		return nil, err
	}
	return httpClientProvider.GetTLSConfig(*opts)
}

func (s *Service) DecryptedValues(ctx context.Context, ds *models.DataSource) (map[string]string, error) {
	decryptedValues := make(map[string]string)
	secret, exist, err := s.SecretsStore.Get(ctx, ds.OrgId, ds.Name, secretType)
	if err != nil {
		return nil, err
	}

	if exist {
		err = json.Unmarshal([]byte(secret), &decryptedValues)
	}

	if (!exist || err != nil) && len(ds.SecureJsonData) > 0 {
		decryptedValues, err = s.MigrateSecrets(ctx, ds)
		if err != nil {
			return nil, err
		}
	}

	return decryptedValues, nil
}

func (s *Service) MigrateSecrets(ctx context.Context, ds *models.DataSource) (map[string]string, error) {
	secureJsonData := make(map[string]string)
	for k, v := range ds.SecureJsonData {
		decrypted, err := s.SecretsService.Decrypt(ctx, v)
		if err != nil {
			return nil, err
		}
		secureJsonData[k] = string(decrypted)
	}

	jsonData, err := json.Marshal(secureJsonData)
	if err != nil {
		return nil, err
	}

	err = s.SecretsStore.Set(ctx, ds.OrgId, ds.Name, secretType, string(jsonData))
	return secureJsonData, err
}

func (s *Service) DecryptedValue(ctx context.Context, ds *models.DataSource, key string) (string, bool, error) {
	values, err := s.DecryptedValues(ctx, ds)
	if err != nil {
		return "", false, err
	}
	value, exists := values[key]
	return value, exists, nil
}

func (s *Service) DecryptedBasicAuthPassword(ctx context.Context, ds *models.DataSource) (string, error) {
	value, ok, err := s.DecryptedValue(ctx, ds, "basicAuthPassword")
	if ok {
		return value, nil
	}

	return "", err
}

func (s *Service) DecryptedPassword(ctx context.Context, ds *models.DataSource) (string, error) {
	value, ok, err := s.DecryptedValue(ctx, ds, "password")
	if ok {
		return value, nil
	}

	return "", err
}

func (s *Service) httpClientOptions(ctx context.Context, ds *models.DataSource) (*sdkhttpclient.Options, error) {
	tlsOptions, err := s.dsTLSOptions(ctx, ds)
	if err != nil {
		return nil, err
	}

	timeouts := &sdkhttpclient.TimeoutOptions{
		Timeout:               s.getTimeout(ds),
		DialTimeout:           sdkhttpclient.DefaultTimeoutOptions.DialTimeout,
		KeepAlive:             sdkhttpclient.DefaultTimeoutOptions.KeepAlive,
		TLSHandshakeTimeout:   sdkhttpclient.DefaultTimeoutOptions.TLSHandshakeTimeout,
		ExpectContinueTimeout: sdkhttpclient.DefaultTimeoutOptions.ExpectContinueTimeout,
		MaxConnsPerHost:       sdkhttpclient.DefaultTimeoutOptions.MaxConnsPerHost,
		MaxIdleConns:          sdkhttpclient.DefaultTimeoutOptions.MaxIdleConns,
		MaxIdleConnsPerHost:   sdkhttpclient.DefaultTimeoutOptions.MaxIdleConnsPerHost,
		IdleConnTimeout:       sdkhttpclient.DefaultTimeoutOptions.IdleConnTimeout,
	}

	decryptedValues, err := s.DecryptedValues(ctx, ds)
	if err != nil {
		return nil, err
	}

	opts := &sdkhttpclient.Options{
		Timeouts: timeouts,
		Headers:  s.getCustomHeaders(ds.JsonData, decryptedValues),
		Labels: map[string]string{
			"datasource_name": ds.Name,
			"datasource_uid":  ds.Uid,
		},
		TLS: &tlsOptions,
	}

	if ds.JsonData != nil {
		opts.CustomOptions = ds.JsonData.MustMap()
	}
	if ds.BasicAuth {
		password, err := s.DecryptedBasicAuthPassword(ctx, ds)
		if err != nil {
			return opts, err
		}

		opts.BasicAuth = &sdkhttpclient.BasicAuthOptions{
			User:     ds.BasicAuthUser,
			Password: password,
		}
	} else if ds.User != "" {
		password, err := s.DecryptedPassword(ctx, ds)
		if err != nil {
			return opts, err
		}

		opts.BasicAuth = &sdkhttpclient.BasicAuthOptions{
			User:     ds.User,
			Password: password,
		}
	}

	// TODO: #35857 Required for templating queries in Prometheus datasource when Azure authentication enabled
	if ds.JsonData != nil && s.features.IsEnabled(featuremgmt.FlagPrometheusAzureAuth) {
		credentials, err := azcredentials.FromDatasourceData(ds.JsonData.MustMap(), decryptedValues)
		if err != nil {
			err = fmt.Errorf("invalid Azure credentials: %s", err)
			return nil, err
		}

		if credentials != nil {
			var scopes []string

			if scopes, err = promclient.GetOverriddenScopes(ds.JsonData.MustMap()); err != nil {
				return nil, err
			}

			if scopes == nil {
				if scopes, err = promclient.GetPrometheusScopes(s.cfg.Azure, credentials); err != nil {
					return nil, err
				}
			}

			azhttpclient.AddAzureAuthentication(opts, s.cfg.Azure, credentials, scopes)
		}
	}

	if ds.JsonData != nil && ds.JsonData.Get("sigV4Auth").MustBool(false) && setting.SigV4AuthEnabled {
		opts.SigV4 = &sdkhttpclient.SigV4Config{
			Service:       awsServiceNamespace(ds.Type),
			Region:        ds.JsonData.Get("sigV4Region").MustString(),
			AssumeRoleARN: ds.JsonData.Get("sigV4AssumeRoleArn").MustString(),
			AuthType:      ds.JsonData.Get("sigV4AuthType").MustString(),
			ExternalID:    ds.JsonData.Get("sigV4ExternalId").MustString(),
			Profile:       ds.JsonData.Get("sigV4Profile").MustString(),
		}

		if val, exists, err := s.DecryptedValue(ctx, ds, "sigV4AccessKey"); err == nil {
			if exists {
				opts.SigV4.AccessKey = val
			}
		} else {
			return opts, err
		}

		if val, exists, err := s.DecryptedValue(ctx, ds, "sigV4SecretKey"); err == nil {
			if exists {
				opts.SigV4.SecretKey = val
			}
		} else {
			return opts, err
		}
	}

	return opts, nil
}

func (s *Service) dsTLSOptions(ctx context.Context, ds *models.DataSource) (sdkhttpclient.TLSOptions, error) {
	var tlsSkipVerify, tlsClientAuth, tlsAuthWithCACert bool
	var serverName string

	if ds.JsonData != nil {
		tlsClientAuth = ds.JsonData.Get("tlsAuth").MustBool(false)
		tlsAuthWithCACert = ds.JsonData.Get("tlsAuthWithCACert").MustBool(false)
		tlsSkipVerify = ds.JsonData.Get("tlsSkipVerify").MustBool(false)
		serverName = ds.JsonData.Get("serverName").MustString()
	}

	opts := sdkhttpclient.TLSOptions{
		InsecureSkipVerify: tlsSkipVerify,
		ServerName:         serverName,
	}

	if tlsClientAuth || tlsAuthWithCACert {
		if tlsAuthWithCACert {
			if val, exists, err := s.DecryptedValue(ctx, ds, "tlsCACert"); err == nil {
				if exists && len(val) > 0 {
					opts.CACertificate = val
				}
			} else {
				return opts, err
			}
		}

		if tlsClientAuth {
			if val, exists, err := s.DecryptedValue(ctx, ds, "tlsClientCert"); err == nil {
				fmt.Print("\n\n\n\n", val, exists, err, "\n\n\n\n")
				if exists && len(val) > 0 {
					opts.ClientCertificate = val
				}
			} else {
				return opts, err
			}
			if val, exists, err := s.DecryptedValue(ctx, ds, "tlsClientKey"); err == nil {
				if exists && len(val) > 0 {
					opts.ClientKey = val
				}
			} else {
				return opts, err
			}
		}
	}

	return opts, nil
}

func (s *Service) getTimeout(ds *models.DataSource) time.Duration {
	timeout := 0
	if ds.JsonData != nil {
		timeout = ds.JsonData.Get("timeout").MustInt()
		if timeout <= 0 {
			if timeoutStr := ds.JsonData.Get("timeout").MustString(); timeoutStr != "" {
				if t, err := strconv.Atoi(timeoutStr); err == nil {
					timeout = t
				}
			}
		}
	}
	if timeout <= 0 {
		return sdkhttpclient.DefaultTimeoutOptions.Timeout
	}

	return time.Duration(timeout) * time.Second
}

// getCustomHeaders returns a map with all the to be set headers
// The map key represents the HeaderName and the value represents this header's value
func (s *Service) getCustomHeaders(jsonData *simplejson.Json, decryptedValues map[string]string) map[string]string {
	headers := make(map[string]string)
	if jsonData == nil {
		return headers
	}

	index := 1
	for {
		headerNameSuffix := fmt.Sprintf("httpHeaderName%d", index)
		headerValueSuffix := fmt.Sprintf("httpHeaderValue%d", index)

		key := jsonData.Get(headerNameSuffix).MustString()
		if key == "" {
			// No (more) header values are available
			break
		}

		if val, ok := decryptedValues[headerValueSuffix]; ok {
			headers[key] = val
		}
		index++
	}

	return headers
}

func awsServiceNamespace(dsType string) string {
	switch dsType {
	case models.DS_ES, models.DS_ES_OPEN_DISTRO, models.DS_ES_OPENSEARCH:
		return "es"
	case models.DS_PROMETHEUS:
		return "aps"
	default:
		panic(fmt.Sprintf("Unsupported datasource %q", dsType))
	}
}

func (s *Service) fillWithSecureJSONData(ctx context.Context, cmd *models.UpdateDataSourceCommand, ds *models.DataSource) error {
	decrypted, err := s.DecryptedValues(ctx, ds)
	if err != nil {
		return err
	}

	if cmd.SecureJsonData == nil {
		cmd.SecureJsonData = make(map[string]string)
	}

	for k, v := range decrypted {
		if _, ok := cmd.SecureJsonData[k]; !ok {
			cmd.SecureJsonData[k] = v
		}
	}

	// this is here for backwards compatibility
	cmd.EncryptedSecureJsonData, err = s.SecretsService.EncryptJsonData(ctx, cmd.SecureJsonData, secrets.WithoutScope())
	if err != nil {
		return err
	}

	return nil
}
