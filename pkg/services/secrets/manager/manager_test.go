package manager

import (
	"context"
	"testing"

	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/services/encryption/ossencryption"
	"github.com/grafana/grafana/pkg/services/kmsproviders/osskmsproviders"
	"github.com/grafana/grafana/pkg/services/secrets"
	"github.com/grafana/grafana/pkg/services/secrets/database"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func TestSecretsService_EnvelopeEncryption(t *testing.T) {
	store := database.ProvideSecretsStore(sqlstore.InitTestDB(t))
	svc := SetupTestService(t, store)
	ctx := context.Background()

	t.Run("encrypting with no entity_id should create DEK", func(t *testing.T) {
		plaintext := []byte("very secret string")

		encrypted, err := svc.Encrypt(context.Background(), plaintext, secrets.WithoutScope())
		require.NoError(t, err)

		decrypted, err := svc.Decrypt(context.Background(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)

		keys, err := store.GetAllDataKeys(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(keys), 1)
	})

	t.Run("encrypting another secret with no entity_id should use the same DEK", func(t *testing.T) {
		plaintext := []byte("another very secret string")

		encrypted, err := svc.Encrypt(context.Background(), plaintext, secrets.WithoutScope())
		require.NoError(t, err)

		decrypted, err := svc.Decrypt(context.Background(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)

		keys, err := store.GetAllDataKeys(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(keys), 1)
	})

	t.Run("encrypting with entity_id provided should create a new DEK", func(t *testing.T) {
		plaintext := []byte("some test data")

		encrypted, err := svc.Encrypt(context.Background(), plaintext, secrets.WithScope("user:100"))
		require.NoError(t, err)

		decrypted, err := svc.Decrypt(context.Background(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)

		keys, err := store.GetAllDataKeys(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(keys), 2)
	})

	t.Run("decrypting empty payload should return error", func(t *testing.T) {
		_, err := svc.Decrypt(context.Background(), []byte(""))
		require.Error(t, err)

		assert.Equal(t, "unable to decrypt empty payload", err.Error())
	})

	t.Run("decrypting legacy secret encrypted with secret key from settings", func(t *testing.T) {
		expected := "grafana"
		encrypted := []byte{122, 56, 53, 113, 101, 117, 73, 89, 20, 254, 36, 112, 112, 16, 128, 232, 227, 52, 166, 108, 192, 5, 28, 125, 126, 42, 197, 190, 251, 36, 94}
		decrypted, err := svc.Decrypt(context.Background(), encrypted)
		require.NoError(t, err)
		assert.Equal(t, expected, string(decrypted))
	})

	t.Run("usage stats should be registered", func(t *testing.T) {
		reports, err := svc.usageStats.GetUsageReport(context.Background())
		require.NoError(t, err)

		assert.Equal(t, 1, reports.Metrics["stats.encryption.envelope_encryption_enabled.count"])
	})
}

func TestSecretsService_DataKeys(t *testing.T) {
	store := database.ProvideSecretsStore(sqlstore.InitTestDB(t))
	ctx := context.Background()

	dataKey := secrets.DataKey{
		Active:        true,
		Name:          "test1",
		Provider:      "test",
		EncryptedData: []byte{0x62, 0xAF, 0xA1, 0x1A},
	}

	t.Run("querying for a DEK that does not exist", func(t *testing.T) {
		res, err := store.GetDataKey(ctx, dataKey.Name)
		assert.ErrorIs(t, secrets.ErrDataKeyNotFound, err)
		assert.Nil(t, res)
	})

	t.Run("creating an active DEK", func(t *testing.T) {
		err := store.CreateDataKey(ctx, dataKey)
		require.NoError(t, err)

		res, err := store.GetDataKey(ctx, dataKey.Name)
		require.NoError(t, err)
		assert.Equal(t, dataKey.EncryptedData, res.EncryptedData)
		assert.Equal(t, dataKey.Provider, res.Provider)
		assert.Equal(t, dataKey.Name, res.Name)
		assert.True(t, dataKey.Active)
	})

	t.Run("creating an inactive DEK", func(t *testing.T) {
		k := secrets.DataKey{
			Active:        false,
			Name:          "test2",
			Provider:      "test",
			EncryptedData: []byte{0x62, 0xAF, 0xA1, 0x1A},
		}

		err := store.CreateDataKey(ctx, k)
		require.Error(t, err)

		res, err := store.GetDataKey(ctx, k.Name)
		assert.Equal(t, secrets.ErrDataKeyNotFound, err)
		assert.Nil(t, res)
	})

	t.Run("deleting DEK when no name provided must fail", func(t *testing.T) {
		beforeDelete, err := store.GetAllDataKeys(ctx)
		require.NoError(t, err)
		err = store.DeleteDataKey(ctx, "")
		require.Error(t, err)

		afterDelete, err := store.GetAllDataKeys(ctx)
		require.NoError(t, err)
		assert.Equal(t, beforeDelete, afterDelete)
	})

	t.Run("deleting a DEK", func(t *testing.T) {
		err := store.DeleteDataKey(ctx, dataKey.Name)
		require.NoError(t, err)

		res, err := store.GetDataKey(ctx, dataKey.Name)
		assert.Equal(t, secrets.ErrDataKeyNotFound, err)
		assert.Nil(t, res)
	})
}

func TestSecretsService_UseCurrentProvider(t *testing.T) {
	t.Run("When encryption_provider is not specified explicitly, should use 'secretKey' as a current provider", func(t *testing.T) {
		svc := SetupTestService(t, database.ProvideSecretsStore(sqlstore.InitTestDB(t)))
		assert.Equal(t, "secretKey", svc.currentProvider)
	})

	t.Run("Should use encrypt/decrypt methods of the current encryption provider", func(t *testing.T) {
		rawCfg := `
		[security]
		secret_key = sdDkslslld
		encryption_provider = fakeProvider.v1
		available_encryption_providers = fakeProvider.v1

		[security.encryption.fakeProvider.v1]
		`

		raw, err := ini.Load([]byte(rawCfg))
		require.NoError(t, err)

		providerID := "fakeProvider.v1"
		settings := &setting.OSSImpl{
			Cfg: &setting.Cfg{
				Raw:            raw,
				FeatureToggles: map[string]bool{secrets.EnvelopeEncryptionFeatureToggle: true},
			},
		}
		encr := ossencryption.ProvideService()
		kms := newFakeKMS(osskmsproviders.ProvideService(encr, settings))
		secretStore := database.ProvideSecretsStore(sqlstore.InitTestDB(t))

		svcEncrypt, err := ProvideSecretsService(
			secretStore,
			&kms,
			encr,
			settings,
			&usagestats.UsageStatsMock{T: t},
		)
		require.NoError(t, err)

		assert.Equal(t, providerID, svcEncrypt.currentProvider)
		assert.Equal(t, 2, len(svcEncrypt.GetProviders()))

		encrypted, _ := svcEncrypt.Encrypt(context.Background(), []byte{}, secrets.WithoutScope())
		assert.True(t, kms.fake.encryptCalled)

		// secret service tries to find a DEK in a cache first before calling provider's decrypt
		// to bypass the cache, we set up one more secrets service to test decrypting
		svcDecrypt, err := ProvideSecretsService(
			secretStore,
			&kms,
			encr,
			settings,
			&usagestats.UsageStatsMock{T: t},
		)
		require.NoError(t, err)

		_, _ = svcDecrypt.Decrypt(context.Background(), encrypted)
		assert.True(t, kms.fake.decryptCalled, "fake provider's decrypt should be called")
	})
}

type fakeProvider struct {
	encryptCalled bool
	decryptCalled bool
}

func (p *fakeProvider) Encrypt(_ context.Context, _ []byte) ([]byte, error) {
	p.encryptCalled = true
	return []byte{}, nil
}

func (p *fakeProvider) Decrypt(_ context.Context, _ []byte) ([]byte, error) {
	p.decryptCalled = true
	return []byte{}, nil
}

type fakeKMS struct {
	kms  osskmsproviders.Service
	fake *fakeProvider
}

func newFakeKMS(kms osskmsproviders.Service) fakeKMS {
	return fakeKMS{
		kms:  kms,
		fake: &fakeProvider{},
	}
}

func (f *fakeKMS) Provide() (map[string]secrets.Provider, error) {
	providers, err := f.kms.Provide()
	if err != nil {
		return providers, err
	}

	providers["fakeProvider.v1"] = f.fake
	return providers, nil
}
