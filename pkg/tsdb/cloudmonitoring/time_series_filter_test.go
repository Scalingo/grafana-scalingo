package cloudmonitoring

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeSeriesFilter(t *testing.T) {
	t.Run("when data from query aggregated to one time series", func(t *testing.T) {
		data, err := loadTestFile("./test-data/1-series-response-agg-one-metric.json")
		require.NoError(t, err)
		assert.Equal(t, 1, len(data.TimeSeries))

		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)
		frames := res.Frames
		require.Len(t, frames, 1)
		assert.Equal(t, "serviceruntime.googleapis.com/api/request_count", frames[0].Fields[1].Name)
		assert.Equal(t, 3, frames[0].Fields[1].Len())

		assert.Equal(t, 0.05, frames[0].Fields[1].At(0))
		assert.Equal(t, time.Unix(int64(1536670020000/1000), 0).UTC(), frames[0].Fields[0].At(0))

		assert.Equal(t, 1.05, frames[0].Fields[1].At(1))
		assert.Equal(t, time.Unix(int64(1536670080000/1000), 0).UTC(), frames[0].Fields[0].At(1))

		assert.Equal(t, 1.0666666666667, frames[0].Fields[1].At(2))
		assert.Equal(t, time.Unix(int64(1536670260000/1000), 0).UTC(), frames[0].Fields[0].At(2))
	})

	t.Run("when data from query with no aggregation", func(t *testing.T) {
		data, err := loadTestFile("./test-data/2-series-response-no-agg.json")
		require.NoError(t, err)
		assert.Equal(t, 3, len(data.TimeSeries))
		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)

		field := res.Frames[0].Fields[1]
		assert.Equal(t, 3, field.Len())
		assert.Equal(t, 9.8566497180145, field.At(0))
		assert.Equal(t, 9.7323568146676, field.At(1))
		assert.Equal(t, 9.7730520330369, field.At(2))
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-asia-east-1", field.Name)
		assert.Equal(t, "collector-asia-east-1", field.Labels["metric.label.instance_name"])
		assert.Equal(t, "asia-east1-a", field.Labels["resource.label.zone"])
		assert.Equal(t, "grafana-prod", field.Labels["resource.label.project_id"])

		field = res.Frames[1].Fields[1]
		assert.Equal(t, 3, field.Len())
		assert.Equal(t, 9.0238475054502, field.At(0))
		assert.Equal(t, 8.9689492364414, field.At(1))
		assert.Equal(t, 8.8210971239023, field.At(2))
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-europe-west-1", field.Name)
		assert.Equal(t, "collector-europe-west-1", field.Labels["metric.label.instance_name"])
		assert.Equal(t, "europe-west1-b", field.Labels["resource.label.zone"])
		assert.Equal(t, "grafana-prod", field.Labels["resource.label.project_id"])

		field = res.Frames[2].Fields[1]
		assert.Equal(t, 3, field.Len())
		assert.Equal(t, 30.829426143318, field.At(0))
		assert.Equal(t, 30.903974115849, field.At(1))
		assert.Equal(t, 30.807846801355, field.At(2))
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-us-east-1", field.Name)
		assert.Equal(t, "collector-us-east-1", field.Labels["metric.label.instance_name"])
		assert.Equal(t, "us-east1-b", field.Labels["resource.label.zone"])
		assert.Equal(t, "grafana-prod", field.Labels["resource.label.project_id"])
	})

	t.Run("when data from query with no aggregation and group bys", func(t *testing.T) {
		data, err := loadTestFile("./test-data/2-series-response-no-agg.json")
		require.NoError(t, err)
		assert.Equal(t, 3, len(data.TimeSeries))
		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, GroupBys: []string{
			"metric.label.instance_name", "resource.label.zone",
		}}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)
		frames := res.Frames
		require.NoError(t, err)

		assert.Equal(t, 3, len(frames))
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-asia-east-1 asia-east1-a", frames[0].Fields[1].Name)
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-europe-west-1 europe-west1-b", frames[1].Fields[1].Name)
		assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time collector-us-east-1 us-east1-b", frames[2].Fields[1].Name)
	})

	t.Run("when data from query with no aggregation and alias by", func(t *testing.T) {
		data, err := loadTestFile("./test-data/2-series-response-no-agg.json")
		require.NoError(t, err)
		assert.Equal(t, 3, len(data.TimeSeries))
		res := &backend.DataResponse{}

		t.Run("and the alias pattern is for metric type, a metric label and a resource label", func(t *testing.T) {
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{metric.type}} - {{metric.label.instance_name}} - {{resource.label.zone}}", GroupBys: []string{"metric.label.instance_name", "resource.label.zone"}}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)

			assert.Equal(t, 3, len(frames))
			assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time - collector-asia-east-1 - asia-east1-a", frames[0].Fields[1].Name)
			assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time - collector-europe-west-1 - europe-west1-b", frames[1].Fields[1].Name)
			assert.Equal(t, "compute.googleapis.com/instance/cpu/usage_time - collector-us-east-1 - us-east1-b", frames[2].Fields[1].Name)
		})

		t.Run("and the alias pattern is for metric name", func(t *testing.T) {
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "metric {{metric.name}} service {{metric.service}}", GroupBys: []string{"metric.label.instance_name", "resource.label.zone"}}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)

			assert.Equal(t, 3, len(frames))
			assert.Equal(t, "metric instance/cpu/usage_time service compute", frames[0].Fields[1].Name)
			assert.Equal(t, "metric instance/cpu/usage_time service compute", frames[1].Fields[1].Name)
			assert.Equal(t, "metric instance/cpu/usage_time service compute", frames[2].Fields[1].Name)
		})
	})

	t.Run("when data from query is distribution with exponential bounds", func(t *testing.T) {
		data, err := loadTestFile("./test-data/3-series-response-distribution-exponential.json")
		require.NoError(t, err)
		assert.Equal(t, 1, len(data.TimeSeries))
		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{bucket}}"}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)
		frames := res.Frames
		require.NoError(t, err)
		assert.Equal(t, 11, len(frames))
		for i := 0; i < 11; i++ {
			if i == 0 {
				assert.Equal(t, "0", frames[i].Fields[1].Name)
			} else {
				assert.Equal(t, strconv.FormatInt(int64(math.Pow(float64(2), float64(i-1))), 10), frames[i].Fields[1].Name)
			}
			assert.Equal(t, 3, frames[i].Fields[0].Len())
		}

		assert.Equal(t, time.Unix(int64(1536668940000/1000), 0).UTC(), frames[0].Fields[0].At(0))
		assert.Equal(t, time.Unix(int64(1536669000000/1000), 0).UTC(), frames[0].Fields[0].At(1))
		assert.Equal(t, time.Unix(int64(1536669060000/1000), 0).UTC(), frames[0].Fields[0].At(2))

		assert.Equal(t, "0", frames[0].Fields[1].Name)
		assert.Equal(t, "1", frames[1].Fields[1].Name)
		assert.Equal(t, "2", frames[2].Fields[1].Name)
		assert.Equal(t, "4", frames[3].Fields[1].Name)
		assert.Equal(t, "8", frames[4].Fields[1].Name)

		assert.Equal(t, float64(1), frames[8].Fields[1].At(0))
		assert.Equal(t, float64(1), frames[9].Fields[1].At(0))
		assert.Equal(t, float64(1), frames[10].Fields[1].At(0))
		assert.Equal(t, float64(0), frames[8].Fields[1].At(1))
		assert.Equal(t, float64(0), frames[9].Fields[1].At(1))
		assert.Equal(t, float64(1), frames[10].Fields[1].At(1))
		assert.Equal(t, float64(0), frames[8].Fields[1].At(2))
		assert.Equal(t, float64(1), frames[9].Fields[1].At(2))
		assert.Equal(t, float64(0), frames[10].Fields[1].At(2))
	})

	t.Run("when data from query is distribution with explicit bounds", func(t *testing.T) {
		data, err := loadTestFile("./test-data/4-series-response-distribution-explicit.json")
		require.NoError(t, err)
		assert.Equal(t, 1, len(data.TimeSeries))
		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{bucket}}"}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)
		frames := res.Frames
		require.NoError(t, err)
		assert.Equal(t, 33, len(frames))
		for i := 0; i < 33; i++ {
			if i == 0 {
				assert.Equal(t, "0", frames[i].Fields[1].Name)
			}
			assert.Equal(t, 2, frames[i].Fields[1].Len())
		}

		assert.Equal(t, time.Unix(int64(1550859086000/1000), 0).UTC(), frames[0].Fields[0].At(0))
		assert.Equal(t, time.Unix(int64(1550859146000/1000), 0).UTC(), frames[0].Fields[0].At(1))

		assert.Equal(t, "0", frames[0].Fields[1].Name)
		assert.Equal(t, "0.01", frames[1].Fields[1].Name)
		assert.Equal(t, "0.05", frames[2].Fields[1].Name)
		assert.Equal(t, "0.1", frames[3].Fields[1].Name)

		assert.Equal(t, float64(381), frames[8].Fields[1].At(0))
		assert.Equal(t, float64(212), frames[9].Fields[1].At(0))
		assert.Equal(t, float64(56), frames[10].Fields[1].At(0))
		assert.Equal(t, float64(375), frames[8].Fields[1].At(1))
		assert.Equal(t, float64(213), frames[9].Fields[1].At(1))
		assert.Equal(t, float64(56), frames[10].Fields[1].At(1))
	})

	t.Run("when data from query returns metadata system labels", func(t *testing.T) {
		data, err := loadTestFile("./test-data/5-series-response-meta-data.json")
		require.NoError(t, err)
		assert.Equal(t, 3, len(data.TimeSeries))
		res := &backend.DataResponse{}
		query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{bucket}}"}
		err = query.parseResponse(res, data, "")
		require.NoError(t, err)
		require.NoError(t, err)
		assert.Equal(t, 3, len(res.Frames))

		field := res.Frames[0].Fields[1]
		assert.Equal(t, "diana-debian9", field.Labels["metadata.system_labels.name"])
		assert.Equal(t, "value1, value2", field.Labels["metadata.system_labels.test"])
		assert.Equal(t, "us-west1", field.Labels["metadata.system_labels.region"])
		assert.Equal(t, "false", field.Labels["metadata.system_labels.spot_instance"])
		assert.Equal(t, "name1", field.Labels["metadata.user_labels.name"])
		assert.Equal(t, "region1", field.Labels["metadata.user_labels.region"])

		field = res.Frames[1].Fields[1]
		assert.Equal(t, "diana-ubuntu1910", field.Labels["metadata.system_labels.name"])
		assert.Equal(t, "value1, value2, value3", field.Labels["metadata.system_labels.test"])
		assert.Equal(t, "us-west1", field.Labels["metadata.system_labels.region"])
		assert.Equal(t, "false", field.Labels["metadata.system_labels.spot_instance"])

		field = res.Frames[2].Fields[1]
		assert.Equal(t, "premium-plugin-staging", field.Labels["metadata.system_labels.name"])
		assert.Equal(t, "value1, value2, value4, value5", field.Labels["metadata.system_labels.test"])
		assert.Equal(t, "us-central1", field.Labels["metadata.system_labels.region"])
		assert.Equal(t, "true", field.Labels["metadata.system_labels.spot_instance"])
		assert.Equal(t, "name3", field.Labels["metadata.user_labels.name"])
		assert.Equal(t, "region3", field.Labels["metadata.user_labels.region"])
	})

	t.Run("when data from query returns metadata system labels and alias by is defined", func(t *testing.T) {
		data, err := loadTestFile("./test-data/5-series-response-meta-data.json")
		require.NoError(t, err)
		assert.Equal(t, 3, len(data.TimeSeries))

		t.Run("and systemlabel contains key with array of string", func(t *testing.T) {
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{metadata.system_labels.test}}"}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, 3, len(frames))
			fmt.Println(frames[0].Fields[1].Name)
			assert.Equal(t, "value1, value2", frames[0].Fields[1].Name)
			assert.Equal(t, "value1, value2, value3", frames[1].Fields[1].Name)
			assert.Equal(t, "value1, value2, value4, value5", frames[2].Fields[1].Name)
		})

		t.Run("and systemlabel contains key with array of string2", func(t *testing.T) {
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}, AliasBy: "{{metadata.system_labels.test2}}"}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, 3, len(frames))
			assert.Equal(t, "testvalue", frames[2].Fields[1].Name)
		})
	})

	t.Run("when data from query returns slo and alias by is defined", func(t *testing.T) {
		data, err := loadTestFile("./test-data/6-series-response-slo.json")
		require.NoError(t, err)
		assert.Equal(t, 1, len(data.TimeSeries))

		t.Run("and alias by is expanded", func(t *testing.T) {
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{
				Params:      url.Values{},
				ProjectName: "test-proj",
				Selector:    "select_slo_compliance",
				Service:     "test-service",
				Slo:         "test-slo",
				AliasBy:     "{{project}} - {{service}} - {{slo}} - {{selector}}",
			}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, "test-proj - test-service - test-slo - select_slo_compliance", frames[0].Fields[1].Name)
		})
	})

	t.Run("when data from query returns slo and alias by is not defined", func(t *testing.T) {
		data, err := loadTestFile("./test-data/6-series-response-slo.json")
		require.NoError(t, err)
		assert.Equal(t, 1, len(data.TimeSeries))

		t.Run("and alias by is expanded", func(t *testing.T) {
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{
				Params:      url.Values{},
				ProjectName: "test-proj",
				Selector:    "select_slo_compliance",
				Service:     "test-service",
				Slo:         "test-slo",
			}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, "select_slo_compliance(\"projects/test-proj/services/test-service/serviceLevelObjectives/test-slo\")", frames[0].Fields[1].Name)
		})
	})

	t.Run("Parse cloud monitoring unit", func(t *testing.T) {
		t.Run("when mapping is found a unit should be specified on the field config", func(t *testing.T) {
			data, err := loadTestFile("./test-data/1-series-response-agg-one-metric.json")
			require.NoError(t, err)
			assert.Equal(t, 1, len(data.TimeSeries))
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, "Bps", frames[0].Fields[1].Config.Unit)
		})

		t.Run("when mapping is found a unit should be specified on the field config", func(t *testing.T) {
			data, err := loadTestFile("./test-data/2-series-response-no-agg.json")
			require.NoError(t, err)
			assert.Equal(t, 3, len(data.TimeSeries))
			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesFilter{Params: url.Values{}}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			require.NoError(t, err)
			assert.Equal(t, "", frames[0].Fields[1].Config.Unit)
		})
	})

	t.Run("when data from query returns MQL and alias by is defined", func(t *testing.T) {
		data, err := loadTestFile("./test-data/7-series-response-mql.json")
		require.NoError(t, err)
		assert.Equal(t, 0, len(data.TimeSeries))
		assert.Equal(t, 1, len(data.TimeSeriesData))

		t.Run("and alias by is expanded", func(t *testing.T) {
			fromStart := time.Date(2018, 3, 15, 13, 0, 0, 0, time.UTC).In(time.Local)

			res := &backend.DataResponse{}
			query := &cloudMonitoringTimeSeriesQuery{
				ProjectName: "test-proj",
				Query:       "test-query",
				AliasBy:     "{{project}} - {{resource.label.zone}} - {{resource.label.instance_id}} - {{metric.label.response_code_class}}",
				timeRange: backend.TimeRange{
					From: fromStart,
					To:   fromStart.Add(34 * time.Minute),
				},
			}
			err = query.parseResponse(res, data, "")
			require.NoError(t, err)
			frames := res.Frames
			assert.Equal(t, "test-proj - asia-northeast1-c - 6724404429462225363 - 200", frames[0].Fields[1].Name)
		})
	})
}

func loadTestFile(path string) (cloudMonitoringResponse, error) {
	var data cloudMonitoringResponse

	// Can ignore gosec warning G304 here since it's a test path
	// nolint:gosec
	jsonBody, err := ioutil.ReadFile(path)
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(jsonBody, &data)
	return data, err
}
