package database

import (
	"context"
	"testing"

	"github.com/grafana/grafana/pkg/components/apikeygen"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/serviceaccounts"
	"github.com/grafana/grafana/pkg/services/serviceaccounts/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_UsageStats(t *testing.T) {
	saToCreate := tests.TestUser{Login: "servicetestwithTeam@admin", IsServiceAccount: true}
	db, store := setupTestDatabase(t)
	sa := tests.SetupUserServiceAccount(t, db, saToCreate)

	keyName := t.Name()
	key, err := apikeygen.New(sa.OrgId, keyName)
	require.NoError(t, err)

	cmd := serviceaccounts.AddServiceAccountTokenCommand{
		Name:          keyName,
		OrgId:         sa.OrgId,
		Key:           key.HashedKey,
		SecondsToLive: 0,
		Result:        &models.ApiKey{},
	}

	err = store.AddServiceAccountToken(context.Background(), sa.Id, &cmd)
	require.NoError(t, err)

	stats, err := store.GetUsageMetrics(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(1), stats["stats.serviceaccounts.count"].(int64))
	assert.Equal(t, int64(1), stats["stats.serviceaccounts.tokens.count"].(int64))
	assert.Equal(t, int64(1), stats["stats.serviceaccounts.enabled.count"].(int64))
}
