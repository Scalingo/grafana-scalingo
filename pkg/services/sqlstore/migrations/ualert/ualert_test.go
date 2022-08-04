package ualert

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/util"
	"github.com/prometheus/alertmanager/pkg/labels"
	"github.com/stretchr/testify/require"
)

var MigTitle = migTitle
var RmMigTitle = rmMigTitle
var ClearMigrationEntryTitle = clearMigrationEntryTitle

type RmMigration = rmMigration

func (m *Matchers) UnmarshalJSON(data []byte) error {
	var lines []string
	if err := json.Unmarshal(data, &lines); err != nil {
		return err
	}
	for _, line := range lines {
		pm, err := labels.ParseMatchers(line)
		if err != nil {
			return err
		}
		*m = append(*m, pm...)
	}
	sort.Sort(labels.Matchers(*m))
	return nil
}

func Test_validateAlertmanagerConfig(t *testing.T) {
	tc := []struct {
		name      string
		receivers []*PostableGrafanaReceiver
		err       error
	}{
		{
			name: "when a slack receiver does not have a valid URL - it should error",
			receivers: []*PostableGrafanaReceiver{
				{
					UID:            util.GenerateShortUID(),
					Name:           "SlackWithBadURL",
					Type:           "slack",
					Settings:       simplejson.NewFromAny(map[string]interface{}{}),
					SecureSettings: map[string]string{"url": invalidUri},
				},
			},
			err: fmt.Errorf("failed to validate receiver \"SlackWithBadURL\" of type \"slack\": invalid URL %q", invalidUri),
		},
		{
			name: "when a slack receiver has an invalid recipient - it should not error",
			receivers: []*PostableGrafanaReceiver{
				{
					UID:            util.GenerateShortUID(),
					Name:           "SlackWithBadRecipient",
					Type:           "slack",
					Settings:       simplejson.NewFromAny(map[string]interface{}{"recipient": "this passes"}),
					SecureSettings: map[string]string{"url": "http://webhook.slack.com/myuser"},
				},
			},
		},
		{
			name: "when the configuration is valid - it should not error",
			receivers: []*PostableGrafanaReceiver{
				{
					UID:            util.GenerateShortUID(),
					Name:           "SlackWithBadURL",
					Type:           "slack",
					Settings:       simplejson.NewFromAny(map[string]interface{}{"recipient": "#a-good-channel"}),
					SecureSettings: map[string]string{"url": "http://webhook.slack.com/myuser"},
				},
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			mg := newTestMigration(t)
			orgID := int64(1)

			config := configFromReceivers(t, tt.receivers)
			require.NoError(t, config.EncryptSecureSettings()) // make sure we encrypt the settings
			err := mg.validateAlertmanagerConfig(orgID, config)
			if tt.err != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func configFromReceivers(t *testing.T, receivers []*PostableGrafanaReceiver) *PostableUserConfig {
	t.Helper()

	return &PostableUserConfig{
		AlertmanagerConfig: PostableApiAlertingConfig{
			Receivers: []*PostableApiReceiver{
				{GrafanaManagedReceivers: receivers},
			},
		},
	}
}

func (c *PostableUserConfig) EncryptSecureSettings() error {
	for _, r := range c.AlertmanagerConfig.Receivers {
		for _, gr := range r.GrafanaManagedReceivers {
			encryptedData := GetEncryptedJsonData(gr.SecureSettings)
			for k, v := range encryptedData {
				gr.SecureSettings[k] = base64.StdEncoding.EncodeToString(v)
			}
		}
	}
	return nil
}

const invalidUri = "�6�M��)uk譹1(�h`$�o�N>mĕ����cS2�dh![ę�	���`csB�!��OSxP�{�"

func Test_getAlertFolderNameFromDashboard(t *testing.T) {
	t.Run("should include full title", func(t *testing.T) {
		dash := &dashboard{
			Uid:   util.GenerateShortUID(),
			Title: "TEST",
		}
		folder := getAlertFolderNameFromDashboard(dash)
		require.Contains(t, folder, dash.Title)
		require.Contains(t, folder, dash.Uid)
	})
	t.Run("should cut title to the length", func(t *testing.T) {
		title := ""
		for {
			title += util.GenerateShortUID()
			if len(title) > MaxFolderName {
				title = title[:MaxFolderName]
				break
			}
		}

		dash := &dashboard{
			Uid:   util.GenerateShortUID(),
			Title: title,
		}
		folder := getAlertFolderNameFromDashboard(dash)
		require.Len(t, folder, MaxFolderName)
		require.Contains(t, folder, dash.Uid)
	})
}
