package service

import (
	"context"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	acmock "github.com/grafana/grafana/pkg/services/accesscontrol/mock"
	datasourceservice "github.com/grafana/grafana/pkg/services/datasources/service"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/oauthtoken"
	"github.com/grafana/grafana/pkg/services/secrets/fakes"
	"github.com/grafana/grafana/pkg/services/secrets/kvstore"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/legacydata"
	"github.com/stretchr/testify/require"
)

func TestHandleRequest(t *testing.T) {
	cfg := &setting.Cfg{}

	t.Run("Should invoke plugin manager QueryData when handling request for query", func(t *testing.T) {
		origOAuthIsOAuthPassThruEnabledFunc := oAuthIsOAuthPassThruEnabledFunc
		oAuthIsOAuthPassThruEnabledFunc = func(oAuthTokenService oauthtoken.OAuthTokenService, ds *models.DataSource) bool {
			return false
		}

		t.Cleanup(func() {
			oAuthIsOAuthPassThruEnabledFunc = origOAuthIsOAuthPassThruEnabledFunc
		})

		client := &fakePluginsClient{}
		var actualReq *backend.QueryDataRequest
		client.QueryDataHandlerFunc = func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			actualReq = req
			return backend.NewQueryDataResponse(), nil
		}
		secretsStore := kvstore.SetupTestService(t)
		secretsService := secretsManager.SetupTestService(t, fakes.NewFakeSecretsStore())
		datasourcePermissions := acmock.NewMockedPermissionsService()
		dsService := datasourceservice.ProvideService(nil, secretsService, secretsStore, cfg, featuremgmt.WithFeatures(), acmock.New(), datasourcePermissions)
		s := ProvideService(client, nil, dsService)

		ds := &models.DataSource{Id: 12, Type: "unregisteredType", JsonData: simplejson.New()}
		req := legacydata.DataQuery{
			TimeRange: &legacydata.DataTimeRange{},
			Queries: []legacydata.DataSubQuery{
				{RefID: "A", DataSource: &models.DataSource{Id: 1, Type: "test"}, Model: simplejson.New()},
				{RefID: "B", DataSource: &models.DataSource{Id: 1, Type: "test"}, Model: simplejson.New()},
			},
		}
		res, err := s.HandleRequest(context.Background(), ds, req)
		require.NoError(t, err)
		require.NotNil(t, actualReq)
		require.NotNil(t, res)
	})
}

func Test_generateRequest(t *testing.T) {
	t.Run("Should attach custom headers to request if present", func(t *testing.T) {
		jsonData := simplejson.New()
		jsonData.Set(headerName+"testOne", "x-test-one")
		jsonData.Set("testOne", "x-test-wrong")
		jsonData.Set(headerName+"testTwo", "x-test-two")

		decryptedJsonData := map[string]string{
			headerValue + "testOne": "secret-value-one",
			headerValue + "testTwo": "secret-value-two",
			"something":             "else",
		}

		ds := &models.DataSource{Id: 12, Type: "unregisteredType", JsonData: jsonData}
		query := legacydata.DataQuery{
			TimeRange: &legacydata.DataTimeRange{},
			Queries: []legacydata.DataSubQuery{
				{RefID: "A", DataSource: &models.DataSource{Id: 1, Type: "test"}, Model: simplejson.New()},
				{RefID: "B", DataSource: &models.DataSource{Id: 1, Type: "test"}, Model: simplejson.New()},
			},
		}

		req, err := generateRequest(context.Background(), ds, decryptedJsonData, query)
		require.NoError(t, err)
		require.NotNil(t, req)
		require.EqualValues(t,
			map[string]string{
				"x-test-one": "secret-value-one",
				"x-test-two": "secret-value-two",
			}, req.Headers)
	})
}

type fakePluginsClient struct {
	plugins.Client
	backend.QueryDataHandlerFunc
}

func (m *fakePluginsClient) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if m.QueryDataHandlerFunc != nil {
		return m.QueryDataHandlerFunc.QueryData(ctx, req)
	}

	return nil, nil
}
