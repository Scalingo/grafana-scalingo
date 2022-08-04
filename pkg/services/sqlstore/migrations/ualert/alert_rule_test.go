package ualert

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"

	"github.com/stretchr/testify/require"
)

func TestMigrateAlertRuleQueries(t *testing.T) {
	tc := []struct {
		name     string
		input    *simplejson.Json
		expected string
		err      error
	}{
		{
			name:     "when a query has a sub query - it is extracted",
			input:    simplejson.NewFromAny(map[string]interface{}{"targetFull": "thisisafullquery", "target": "ahalfquery"}),
			expected: `{"target":"thisisafullquery"}`,
		},
		{
			name:     "when a query does not have a sub query - it no-ops",
			input:    simplejson.NewFromAny(map[string]interface{}{"target": "ahalfquery"}),
			expected: `{"target":"ahalfquery"}`,
		},
		{
			name:     "when query was hidden, it removes the flag",
			input:    simplejson.NewFromAny(map[string]interface{}{"hide": true}),
			expected: `{}`,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			model, err := tt.input.Encode()
			require.NoError(t, err)
			queries, err := migrateAlertRuleQueries([]alertQuery{{Model: model}})
			if tt.err != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.err.Error())
				return
			}

			require.NoError(t, err)
			r, err := queries[0].Model.MarshalJSON()
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(r))
		})
	}
}

func TestAddMigrationInfo(t *testing.T) {
	tt := []struct {
		name                string
		tagsJSON            string
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name:                "when alert rule tags are a JSON array, they're ignored.",
			tagsJSON:            `{ "alertRuleTags": ["one", "two", "three", "four"] }`,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"__alertId__": "0", "__dashboardUid__": "", "__panelId__": "0"},
		},
		{
			name:                "when alert rule tags are a JSON object",
			tagsJSON:            `{ "alertRuleTags": { "key": "value", "key2": "value2" } }`,
			expectedLabels:      map[string]string{"key": "value", "key2": "value2"},
			expectedAnnotations: map[string]string{"__alertId__": "0", "__dashboardUid__": "", "__panelId__": "0"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var settings dashAlertSettings
			require.NoError(t, json.Unmarshal([]byte(tc.tagsJSON), &settings))

			labels, annotations := addMigrationInfo(&dashAlert{ParsedSettings: &settings})
			require.Equal(t, tc.expectedLabels, labels)
			require.Equal(t, tc.expectedAnnotations, annotations)
		})
	}
}
