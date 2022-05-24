package alerting

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/ngalert/notifier"
	"github.com/grafana/grafana/pkg/tests/testinfra"
)

func TestAvailableChannels(t *testing.T) {
	_, err := tracing.InitializeTracerForTest()
	require.NoError(t, err)

	dir, path := testinfra.CreateGrafDir(t, testinfra.GrafanaOpts{
		DisableLegacyAlerting: true,
		EnableUnifiedAlerting: true,
		DisableAnonymous:      true,
		AppModeProduction:     true,
	})

	grafanaListedAddr, store := testinfra.StartGrafana(t, dir, path)

	// Create a user to make authenticated requests
	createUser(t, store, models.CreateUserCommand{
		DefaultOrgRole: string(models.ROLE_EDITOR),
		Password:       "password",
		Login:          "grafana",
	})

	alertsURL := fmt.Sprintf("http://grafana:password@%s/api/alert-notifiers", grafanaListedAddr)
	// nolint:gosec
	resp, err := http.Get(alertsURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := resp.Body.Close()
		require.NoError(t, err)
	})
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	expNotifiers := notifier.GetAvailableNotifiers()
	expJson, err := json.Marshal(expNotifiers)
	require.NoError(t, err)
	require.Equal(t, string(expJson), string(b))
}
