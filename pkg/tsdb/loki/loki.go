package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana/pkg/infra/httpclient"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"go.opentelemetry.io/otel/attribute"
)

type Service struct {
	im     instancemgmt.InstanceManager
	plog   log.Logger
	tracer tracing.Tracer
}

var (
	_ backend.QueryDataHandler    = (*Service)(nil)
	_ backend.StreamHandler       = (*Service)(nil)
	_ backend.CallResourceHandler = (*Service)(nil)
)

func ProvideService(httpClientProvider httpclient.Provider, tracer tracing.Tracer) *Service {
	return &Service{
		im:     datasource.NewInstanceManager(newInstanceSettings(httpClientProvider)),
		plog:   log.New("tsdb.loki"),
		tracer: tracer,
	}
}

var (
	legendFormat = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)
)

type datasourceInfo struct {
	HTTPClient    *http.Client
	URL           string
	OauthPassThru bool

	// open streams
	streams   map[string]data.FrameJSONCache
	streamsMu sync.RWMutex
}

type QueryJSONModel struct {
	QueryType    string `json:"queryType"`
	Expr         string `json:"expr"`
	Direction    string `json:"direction"`
	LegendFormat string `json:"legendFormat"`
	Interval     string `json:"interval"`
	IntervalMS   int    `json:"intervalMS"`
	Resolution   int64  `json:"resolution"`
	MaxLines     int    `json:"maxLines"`
	VolumeQuery  bool   `json:"volumeQuery"`
}

type DataSourceJSONModel struct {
	OauthPassThru bool `json:"oauthPassThru"`
}

func parseQueryModel(raw json.RawMessage) (*QueryJSONModel, error) {
	model := &QueryJSONModel{}
	err := json.Unmarshal(raw, model)
	return model, err
}

func newInstanceSettings(httpClientProvider httpclient.Provider) datasource.InstanceFactoryFunc {
	return func(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		opts, err := settings.HTTPClientOptions()
		if err != nil {
			return nil, err
		}

		client, err := httpClientProvider.New(opts)
		if err != nil {
			return nil, err
		}

		jsonModel := DataSourceJSONModel{}
		err = json.Unmarshal(settings.JSONData, &jsonModel)
		if err != nil {
			return nil, err
		}

		model := &datasourceInfo{
			HTTPClient:    client,
			URL:           settings.URL,
			OauthPassThru: jsonModel.OauthPassThru,
			streams:       make(map[string]data.FrameJSONCache),
		}
		return model, nil
	}
}

func getOauthTokenForQueryData(dsInfo *datasourceInfo, headers map[string]string) string {
	if !dsInfo.OauthPassThru {
		return ""
	}

	return headers["Authorization"]
}

func getOauthTokenForCallResource(dsInfo *datasourceInfo, headers map[string][]string) string {
	if !dsInfo.OauthPassThru {
		return ""
	}

	accessValues := headers["Authorization"]

	if len(accessValues) == 0 {
		return ""
	}

	return accessValues[0]
}

func (s *Service) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	dsInfo, err := s.getDSInfo(req.PluginContext)
	if err != nil {
		return err
	}

	return callResource(ctx, req, sender, dsInfo, s.plog)
}

func callResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender, dsInfo *datasourceInfo, plog log.Logger) error {
	url := req.URL

	// a very basic is-this-url-valid check
	if req.Method != "GET" {
		return fmt.Errorf("invalid resource method: %s", req.Method)
	}
	if (!strings.HasPrefix(url, "labels?")) &&
		(!strings.HasPrefix(url, "label/")) && // the `/label/$label_name/values` form
		(!strings.HasPrefix(url, "series?")) {
		return fmt.Errorf("invalid resource URL: %s", url)
	}
	lokiURL := fmt.Sprintf("/loki/api/v1/%s", url)

	api := newLokiAPI(dsInfo.HTTPClient, dsInfo.URL, plog, getOauthTokenForCallResource(dsInfo, req.Headers))
	bytes, err := api.RawQuery(ctx, lokiURL)

	if err != nil {
		return err
	}

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Headers: map[string][]string{
			"content-type": {"application/json"},
		},
		Body: bytes,
	})
}

func (s *Service) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	dsInfo, err := s.getDSInfo(req.PluginContext)
	if err != nil {
		result := backend.NewQueryDataResponse()
		return result, err
	}

	return queryData(ctx, req, dsInfo, s.plog, s.tracer)
}

func queryData(ctx context.Context, req *backend.QueryDataRequest, dsInfo *datasourceInfo, plog log.Logger, tracer tracing.Tracer) (*backend.QueryDataResponse, error) {
	result := backend.NewQueryDataResponse()

	api := newLokiAPI(dsInfo.HTTPClient, dsInfo.URL, plog, getOauthTokenForQueryData(dsInfo, req.Headers))

	queries, err := parseQuery(req)
	if err != nil {
		return result, err
	}

	for _, query := range queries {
		plog.Debug("Sending query", "start", query.Start, "end", query.End, "step", query.Step, "query", query.Expr)
		_, span := tracer.Start(ctx, "alerting.loki")
		span.SetAttributes("expr", query.Expr, attribute.Key("expr").String(query.Expr))
		span.SetAttributes("start_unixnano", query.Start, attribute.Key("start_unixnano").Int64(query.Start.UnixNano()))
		span.SetAttributes("stop_unixnano", query.End, attribute.Key("stop_unixnano").Int64(query.End.UnixNano()))
		defer span.End()

		frames, err := runQuery(ctx, api, query)

		queryRes := backend.DataResponse{}

		if err != nil {
			queryRes.Error = err
		} else {
			queryRes.Frames = frames
		}

		result.Responses[query.RefID] = queryRes
	}
	return result, nil
}

// we extracted this part of the functionality to make it easy to unit-test it
func runQuery(ctx context.Context, api *LokiAPI, query *lokiQuery) (data.Frames, error) {
	frames, err := api.DataQuery(ctx, *query)
	if err != nil {
		return data.Frames{}, err
	}

	for _, frame := range frames {
		if err = adjustFrame(frame, query); err != nil {
			return data.Frames{}, err
		}
		if err != nil {
			return data.Frames{}, err
		}
	}

	return frames, nil
}

func (s *Service) getDSInfo(pluginCtx backend.PluginContext) (*datasourceInfo, error) {
	i, err := s.im.Get(pluginCtx)
	if err != nil {
		return nil, err
	}

	instance, ok := i.(*datasourceInfo)
	if !ok {
		return nil, fmt.Errorf("failed to cast datsource info")
	}

	return instance, nil
}
