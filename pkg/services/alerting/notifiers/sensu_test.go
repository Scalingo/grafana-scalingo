package notifiers

import (
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/encryption/ossencryption"

	"github.com/stretchr/testify/require"
)

func TestSensuNotifier(t *testing.T) {
	t.Run("Parsing alert notification from settings", func(t *testing.T) {
		t.Run("empty settings should return error", func(t *testing.T) {
			json := `{ }`

			settingsJSON, _ := simplejson.NewJson([]byte(json))
			model := &models.AlertNotification{
				Name:     "sensu",
				Type:     "sensu",
				Settings: settingsJSON,
			}

			_, err := NewSensuNotifier(model, ossencryption.ProvideService().GetDecryptedValue)
			require.Error(t, err)
		})

		t.Run("from settings", func(t *testing.T) {
			json := `
				{
					"url": "http://sensu-api.example.com:4567/results",
					"source": "grafana_instance_01",
					"handler": "myhandler"
				}`

			settingsJSON, _ := simplejson.NewJson([]byte(json))
			model := &models.AlertNotification{
				Name:     "sensu",
				Type:     "sensu",
				Settings: settingsJSON,
			}

			not, err := NewSensuNotifier(model, ossencryption.ProvideService().GetDecryptedValue)
			sensuNotifier := not.(*SensuNotifier)

			require.Nil(t, err)
			require.Equal(t, "sensu", sensuNotifier.Name)
			require.Equal(t, "sensu", sensuNotifier.Type)
			require.Equal(t, "http://sensu-api.example.com:4567/results", sensuNotifier.URL)
			require.Equal(t, "grafana_instance_01", sensuNotifier.Source)
			require.Equal(t, "myhandler", sensuNotifier.Handler)
		})
	})
}
