package datasources

import (
	"context"
	"os"
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/util"

	"github.com/stretchr/testify/require"
)

var (
	logger log.Logger = log.New("fake.log")

	twoDatasourcesConfig            = "testdata/two-datasources"
	twoDatasourcesConfigPurgeOthers = "testdata/insert-two-delete-two"
	deleteOneDatasource             = "testdata/delete-one"
	doubleDatasourcesConfig         = "testdata/double-default"
	allProperties                   = "testdata/all-properties"
	versionZero                     = "testdata/version-0"
	brokenYaml                      = "testdata/broken-yaml"
	multipleOrgsWithDefault         = "testdata/multiple-org-default"
	withoutDefaults                 = "testdata/appliedDefaults"
	invalidAccess                   = "testdata/invalid-access"
)

func TestDatasourceAsConfig(t *testing.T) {
	t.Run("when some values missing should apply default on insert", func(t *testing.T) {
		store := &spyStore{}
		orgStore := &mockOrgStore{ExpectedOrg: &models.Org{Id: 1}}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), withoutDefaults)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.inserted), 1)
		require.Equal(t, store.inserted[0].OrgId, int64(1))
		require.Equal(t, store.inserted[0].Access, models.DsAccess("proxy"))
		require.Equal(t, store.inserted[0].Name, "My datasource name")
		require.Equal(t, store.inserted[0].Uid, "P2AD1F727255C56BA")
	})

	t.Run("when some values missing should not change UID when updates", func(t *testing.T) {
		store := &spyStore{
			items: []*models.DataSource{{Name: "My datasource name", OrgId: 1, Id: 1, Uid: util.GenerateShortUID()}},
		}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), withoutDefaults)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.deleted), 0)
		require.Equal(t, len(store.inserted), 0)
		require.Equal(t, len(store.updated), 1)
		require.Equal(t, "", store.updated[0].Uid) // XORM will not update the field if its value is default
	})

	t.Run("no datasource in database", func(t *testing.T) {
		store := &spyStore{}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), twoDatasourcesConfig)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.deleted), 0)
		require.Equal(t, len(store.inserted), 2)
		require.Equal(t, len(store.updated), 0)
	})

	t.Run("One datasource in database with same name should update one datasource", func(t *testing.T) {
		store := &spyStore{items: []*models.DataSource{{Name: "Graphite", OrgId: 1, Id: 1}}}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), twoDatasourcesConfig)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.deleted), 0)
		require.Equal(t, len(store.inserted), 1)
		require.Equal(t, len(store.updated), 1)
	})

	t.Run("Two datasources with is_default should raise error", func(t *testing.T) {
		store := &spyStore{}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), doubleDatasourcesConfig)
		require.Equal(t, err, ErrInvalidConfigToManyDefault)
	})

	t.Run("Multiple datasources in different organizations with isDefault in each organization should not raise error", func(t *testing.T) {
		store := &spyStore{}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), multipleOrgsWithDefault)
		require.NoError(t, err)
		require.Equal(t, len(store.inserted), 4)
		require.True(t, store.inserted[0].IsDefault)
		require.Equal(t, store.inserted[0].OrgId, int64(1))
		require.True(t, store.inserted[2].IsDefault)
		require.Equal(t, store.inserted[2].OrgId, int64(2))
	})

	t.Run("Remove one datasource should have removed old datasource", func(t *testing.T) {
		store := &spyStore{}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), deleteOneDatasource)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, 1, len(store.deleted))
		// should have set OrgID to 1
		require.Equal(t, store.deleted[0].OrgID, int64(1))
		require.Equal(t, 0, len(store.inserted))
		require.Equal(t, len(store.updated), 0)
	})

	t.Run("Two configured datasource and purge others", func(t *testing.T) {
		store := &spyStore{items: []*models.DataSource{{Name: "old-graphite", OrgId: 1, Id: 1}, {Name: "old-graphite2", OrgId: 1, Id: 2}}}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), twoDatasourcesConfigPurgeOthers)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.deleted), 2)
		require.Equal(t, len(store.inserted), 2)
		require.Equal(t, len(store.updated), 0)
	})

	t.Run("Two configured datasource and purge others = false", func(t *testing.T) {
		store := &spyStore{items: []*models.DataSource{{Name: "Graphite", OrgId: 1, Id: 1}, {Name: "old-graphite2", OrgId: 1, Id: 2}}}
		orgStore := &mockOrgStore{}
		dc := newDatasourceProvisioner(logger, store, orgStore)
		err := dc.applyChanges(context.Background(), twoDatasourcesConfig)
		if err != nil {
			t.Fatalf("applyChanges return an error %v", err)
		}

		require.Equal(t, len(store.deleted), 0)
		require.Equal(t, len(store.inserted), 1)
		require.Equal(t, len(store.updated), 1)
	})

	t.Run("broken yaml should return error", func(t *testing.T) {
		reader := &configReader{}
		_, err := reader.readConfig(context.Background(), brokenYaml)
		require.NotNil(t, err)
	})

	t.Run("invalid access should warn about invalid value and return 'proxy'", func(t *testing.T) {
		reader := &configReader{log: logger, orgStore: &mockOrgStore{}}
		configs, err := reader.readConfig(context.Background(), invalidAccess)
		require.NoError(t, err)
		require.Equal(t, configs[0].Datasources[0].Access, models.DS_ACCESS_PROXY)
	})

	t.Run("skip invalid directory", func(t *testing.T) {
		cfgProvider := &configReader{log: log.New("test logger"), orgStore: &mockOrgStore{}}
		cfg, err := cfgProvider.readConfig(context.Background(), "./invalid-directory")
		if err != nil {
			t.Fatalf("readConfig return an error %v", err)
		}

		require.Equal(t, len(cfg), 0)
	})

	t.Run("can read all properties from version 1", func(t *testing.T) {
		_ = os.Setenv("TEST_VAR", "name")
		cfgProvider := &configReader{log: log.New("test logger"), orgStore: &mockOrgStore{}}
		cfg, err := cfgProvider.readConfig(context.Background(), allProperties)
		_ = os.Unsetenv("TEST_VAR")
		if err != nil {
			t.Fatalf("readConfig return an error %v", err)
		}

		require.Equal(t, len(cfg), 3)

		dsCfg := cfg[0]

		require.Equal(t, dsCfg.APIVersion, int64(1))

		validateDatasourceV1(t, dsCfg)
		validateDeleteDatasources(t, dsCfg)

		dsCount := 0
		delDsCount := 0

		for _, c := range cfg {
			dsCount += len(c.Datasources)
			delDsCount += len(c.DeleteDatasources)
		}

		require.Equal(t, dsCount, 2)
		require.Equal(t, delDsCount, 1)
	})

	t.Run("can read all properties from version 0", func(t *testing.T) {
		cfgProvider := &configReader{log: log.New("test logger"), orgStore: &mockOrgStore{}}
		cfg, err := cfgProvider.readConfig(context.Background(), versionZero)
		if err != nil {
			t.Fatalf("readConfig return an error %v", err)
		}

		require.Equal(t, len(cfg), 1)

		dsCfg := cfg[0]

		require.Equal(t, dsCfg.APIVersion, int64(0))

		validateDatasource(t, dsCfg)
		validateDeleteDatasources(t, dsCfg)
	})
}

func validateDeleteDatasources(t *testing.T, dsCfg *configs) {
	require.Equal(t, len(dsCfg.DeleteDatasources), 1)
	deleteDs := dsCfg.DeleteDatasources[0]
	require.Equal(t, deleteDs.Name, "old-graphite3")
	require.Equal(t, deleteDs.OrgID, int64(2))
}

func validateDatasource(t *testing.T, dsCfg *configs) {
	ds := dsCfg.Datasources[0]
	require.Equal(t, ds.Name, "name")
	require.Equal(t, ds.Type, "type")
	require.Equal(t, ds.Access, models.DS_ACCESS_PROXY)
	require.Equal(t, ds.OrgID, int64(2))
	require.Equal(t, ds.URL, "url")
	require.Equal(t, ds.User, "user")
	require.Equal(t, ds.Database, "database")
	require.True(t, ds.BasicAuth)
	require.Equal(t, ds.BasicAuthUser, "basic_auth_user")
	require.True(t, ds.WithCredentials)
	require.True(t, ds.IsDefault)
	require.True(t, ds.Editable)
	require.Equal(t, ds.Version, 10)

	require.Greater(t, len(ds.JSONData), 2)
	require.Equal(t, ds.JSONData["graphiteVersion"], "1.1")
	require.Equal(t, ds.JSONData["tlsAuth"], true)
	require.Equal(t, ds.JSONData["tlsAuthWithCACert"], true)

	require.Greater(t, len(ds.SecureJSONData), 2)
	require.Equal(t, ds.SecureJSONData["tlsCACert"], "MjNOcW9RdkbUDHZmpco2HCYzVq9dE+i6Yi+gmUJotq5CDA==")
	require.Equal(t, ds.SecureJSONData["tlsClientCert"], "ckN0dGlyMXN503YNfjTcf9CV+GGQneN+xmAclQ==")
	require.Equal(t, ds.SecureJSONData["tlsClientKey"], "ZkN4aG1aNkja/gKAB1wlnKFIsy2SRDq4slrM0A==")
}

func validateDatasourceV1(t *testing.T, dsCfg *configs) {
	validateDatasource(t, dsCfg)
	ds := dsCfg.Datasources[0]
	require.Equal(t, ds.UID, "test_uid")
}

type mockOrgStore struct{ ExpectedOrg *models.Org }

func (m *mockOrgStore) GetOrgById(c context.Context, cmd *models.GetOrgByIdQuery) error {
	cmd.Result = m.ExpectedOrg
	return nil
}

type spyStore struct {
	inserted []*models.AddDataSourceCommand
	deleted  []*models.DeleteDataSourceCommand
	updated  []*models.UpdateDataSourceCommand
	items    []*models.DataSource
}

func (s *spyStore) GetDataSource(ctx context.Context, query *models.GetDataSourceQuery) error {
	for _, v := range s.items {
		if query.Name == v.Name && query.OrgId == v.OrgId {
			query.Result = v
			return nil
		}
	}
	return models.ErrDataSourceNotFound
}

func (s *spyStore) DeleteDataSource(ctx context.Context, cmd *models.DeleteDataSourceCommand) error {
	s.deleted = append(s.deleted, cmd)
	return nil
}

func (s *spyStore) AddDataSource(ctx context.Context, cmd *models.AddDataSourceCommand) error {
	s.inserted = append(s.inserted, cmd)
	return nil
}

func (s *spyStore) UpdateDataSource(ctx context.Context, cmd *models.UpdateDataSourceCommand) error {
	s.updated = append(s.updated, cmd)
	return nil
}
