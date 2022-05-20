package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/tests/testinfra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	usernameAdmin    = "admin"
	usernameNonAdmin = "nonAdmin"
	defaultPassword  = "password"
)

var updateSnapshotFlag = false

func TestPlugins(t *testing.T) {
	dir, cfgPath := testinfra.CreateGrafDir(t, testinfra.GrafanaOpts{
		PluginAdminEnabled: true,
	})

	grafanaListedAddr, store := testinfra.StartGrafana(t, dir, cfgPath)

	type testCase struct {
		desc        string
		url         string
		expStatus   int
		expRespPath string
	}

	t.Run("Install", func(t *testing.T) {
		createUser(t, store, models.CreateUserCommand{Login: usernameNonAdmin, Password: defaultPassword, IsAdmin: false})
		createUser(t, store, models.CreateUserCommand{Login: usernameAdmin, Password: defaultPassword, IsAdmin: true})

		t.Run("Request is forbidden if not from an admin", func(t *testing.T) {
			status, body := makePostRequest(t, grafanaAPIURL(usernameNonAdmin, grafanaListedAddr, "plugins/grafana-plugin/install"))
			assert.Equal(t, 403, status)
			assert.Equal(t, "Permission denied", body["message"])

			status, body = makePostRequest(t, grafanaAPIURL(usernameNonAdmin, grafanaListedAddr, "plugins/grafana-plugin/uninstall"))
			assert.Equal(t, 403, status)
			assert.Equal(t, "Permission denied", body["message"])
		})

		t.Run("Request is not forbidden if from an admin", func(t *testing.T) {
			statusCode, body := makePostRequest(t, grafanaAPIURL(usernameAdmin, grafanaListedAddr, "plugins/test/install"))

			assert.Equal(t, 404, statusCode)
			assert.Equal(t, "Plugin not found", body["message"])

			statusCode, body = makePostRequest(t, grafanaAPIURL(usernameAdmin, grafanaListedAddr, "plugins/test/uninstall"))
			assert.Equal(t, 404, statusCode)
			assert.Equal(t, "Plugin not installed", body["message"])
		})
	})

	t.Run("List", func(t *testing.T) {
		testCases := []testCase{
			{
				desc:        "should return all loaded core and bundled plugins",
				url:         "http://%s/api/plugins",
				expStatus:   http.StatusOK,
				expRespPath: "expectedListResp.json",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				url := fmt.Sprintf(tc.url, grafanaListedAddr)
				// nolint:gosec
				resp, err := http.Get(url)
				t.Cleanup(func() {
					require.NoError(t, resp.Body.Close())
				})
				require.NoError(t, err)
				require.Equal(t, tc.expStatus, resp.StatusCode)
				b, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, err)

				expResp := expectedResp(t, tc.expRespPath)

				same := assert.JSONEq(t, expResp, string(b))
				if !same {
					if updateSnapshotFlag {
						t.Log("updating snapshot results")
						updateRespSnapshot(t, tc.expRespPath, string(b))
					}
					t.FailNow()
				}
			})
		}
	})
}

func createUser(t *testing.T, store *sqlstore.SQLStore, cmd models.CreateUserCommand) {
	t.Helper()

	_, err := store.CreateUser(context.Background(), cmd)
	require.NoError(t, err)
}

func makePostRequest(t *testing.T, URL string) (int, map[string]interface{}) {
	t.Helper()

	// nolint:gosec
	resp, err := http.Post(URL, "application/json", bytes.NewBufferString(""))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = resp.Body.Close()
		fmt.Printf("Failed to close response body err: %s", err)
	})
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	var body = make(map[string]interface{})
	err = json.Unmarshal(b, &body)
	require.NoError(t, err)

	return resp.StatusCode, body
}

func grafanaAPIURL(username string, grafanaListedAddr string, path string) string {
	return fmt.Sprintf("http://%s:%s@%s/api/%s", username, defaultPassword, grafanaListedAddr, path)
}

func expectedResp(t *testing.T, filename string) string {
	//nolint:GOSEC
	contents, err := ioutil.ReadFile(filepath.Join("data", filename))
	if err != nil {
		t.Errorf("failed to load %s: %v", filename, err)
	}

	return string(contents)
}

func updateRespSnapshot(t *testing.T, filename string, body string) {
	err := ioutil.WriteFile(filepath.Join("data", filename), []byte(body), 0600)
	if err != nil {
		t.Errorf("error writing snapshot %s: %v", filename, err)
	}
}
