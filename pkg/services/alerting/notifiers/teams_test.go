package notifiers

import (
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/encryption/ossencryption"

	"github.com/stretchr/testify/require"
)

func TestTeamsNotifier(t *testing.T) {
	t.Run("Parsing alert notification from settings", func(t *testing.T) {
		t.Run("empty settings should return error", func(t *testing.T) {
			json := `{ }`

			settingsJSON, _ := simplejson.NewJson([]byte(json))
			model := &models.AlertNotification{
				Name:     "ops",
				Type:     "teams",
				Settings: settingsJSON,
			}

			_, err := NewTeamsNotifier(model, ossencryption.ProvideService().GetDecryptedValue)
			require.Error(t, err)
		})

		t.Run("from settings", func(t *testing.T) {
			json := `
				{
          "url": "http://google.com"
				}`

			settingsJSON, _ := simplejson.NewJson([]byte(json))
			model := &models.AlertNotification{
				Name:     "ops",
				Type:     "teams",
				Settings: settingsJSON,
			}

			not, err := NewTeamsNotifier(model, ossencryption.ProvideService().GetDecryptedValue)
			teamsNotifier := not.(*TeamsNotifier)

			require.Nil(t, err)
			require.Equal(t, "ops", teamsNotifier.Name)
			require.Equal(t, "teams", teamsNotifier.Type)
			require.Equal(t, "http://google.com", teamsNotifier.URL)
		})

		t.Run("from settings with Recipient and Mention", func(t *testing.T) {
			json := `
				{
          "url": "http://google.com"
				}`

			settingsJSON, _ := simplejson.NewJson([]byte(json))
			model := &models.AlertNotification{
				Name:     "ops",
				Type:     "teams",
				Settings: settingsJSON,
			}

			not, err := NewTeamsNotifier(model, ossencryption.ProvideService().GetDecryptedValue)
			teamsNotifier := not.(*TeamsNotifier)

			require.Nil(t, err)
			require.Equal(t, "ops", teamsNotifier.Name)
			require.Equal(t, "teams", teamsNotifier.Type)
			require.Equal(t, "http://google.com", teamsNotifier.URL)
		})
	})
}
