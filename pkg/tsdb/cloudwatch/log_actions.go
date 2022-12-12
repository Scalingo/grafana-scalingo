package cloudwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
)

const (
	limitExceededException = "LimitExceededException"
	defaultEventLimit      = int64(10)
	defaultLogGroupLimit   = int64(50)
)

type AWSError struct {
	Code    string
	Message string
	Payload map[string]string
}

type LogQueryJson struct {
	LogType            string `json:"type"`
	SubType            string
	Limit              *int64
	Time               int64
	StartTime          *int64
	EndTime            *int64
	LogGroupName       string
	LogGroupNames      []string
	LogGroups          []suggestData
	LogGroupNamePrefix string
	LogStreamName      string
	StartFromHead      bool
	Region             string
	QueryString        string
	QueryId            string
	StatsGroups        []string
	Subtype            string
	Expression         string
}

func (e *AWSError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *cloudWatchExecutor) executeLogActions(ctx context.Context, logger log.Logger, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()

	resultChan := make(chan backend.Responses, len(req.Queries))
	eg, ectx := errgroup.WithContext(ctx)

	for _, query := range req.Queries {
		var model LogQueryJson
		err := json.Unmarshal(query.JSON, &model)
		if err != nil {
			return nil, err
		}

		query := query
		eg.Go(func() error {
			dataframe, err := e.executeLogAction(ectx, logger, model, query, req.PluginContext)
			if err != nil {
				var AWSError *AWSError
				if errors.As(err, &AWSError) {
					resultChan <- backend.Responses{
						query.RefID: backend.DataResponse{Frames: data.Frames{}, Error: AWSError},
					}
					return nil
				}
				return err
			}

			groupedFrames, err := groupResponseFrame(dataframe, model.StatsGroups)
			if err != nil {
				return err
			}
			resultChan <- backend.Responses{
				query.RefID: backend.DataResponse{Frames: groupedFrames},
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	close(resultChan)

	for result := range resultChan {
		for refID, response := range result {
			respD := resp.Responses[refID]
			respD.Frames = response.Frames
			respD.Error = response.Error
			resp.Responses[refID] = respD
		}
	}

	return resp, nil
}

func (e *cloudWatchExecutor) executeLogAction(ctx context.Context, logger log.Logger, model LogQueryJson, query backend.DataQuery, pluginCtx backend.PluginContext) (*data.Frame, error) {
	instance, err := e.getInstance(pluginCtx)
	if err != nil {
		return nil, err
	}

	region := instance.Settings.Region
	if model.Region != "" {
		region = model.Region
	}

	logsClient, err := e.getCWLogsClient(pluginCtx, region)
	if err != nil {
		return nil, err
	}

	var data *data.Frame = nil
	switch model.SubType {
	case "GetLogGroupFields":
		data, err = e.handleGetLogGroupFields(ctx, logsClient, model, query.RefID)
	case "StartQuery":
		data, err = e.handleStartQuery(ctx, logger, logsClient, model, query.TimeRange, query.RefID)
	case "StopQuery":
		data, err = e.handleStopQuery(ctx, logsClient, model)
	case "GetQueryResults":
		data, err = e.handleGetQueryResults(ctx, logsClient, model, query.RefID)
	case "GetLogEvents":
		data, err = e.handleGetLogEvents(ctx, logsClient, model)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to execute log action with subtype: %s: %w", model.SubType, err)
	}

	return data, nil
}

func (e *cloudWatchExecutor) handleGetLogEvents(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson) (*data.Frame, error) {
	limit := defaultEventLimit
	if parameters.Limit != nil && *parameters.Limit > 0 {
		limit = *parameters.Limit
	}

	queryRequest := &cloudwatchlogs.GetLogEventsInput{
		Limit:         aws.Int64(limit),
		StartFromHead: aws.Bool(parameters.StartFromHead),
	}

	if parameters.LogGroupName == "" {
		return nil, fmt.Errorf("Error: Parameter 'logGroupName' is required")
	}
	queryRequest.SetLogGroupName(parameters.LogGroupName)

	if parameters.LogStreamName == "" {
		return nil, fmt.Errorf("Error: Parameter 'logStreamName' is required")
	}
	queryRequest.SetLogStreamName(parameters.LogStreamName)

	if parameters.StartTime != nil && *parameters.StartTime != 0 {
		queryRequest.SetStartTime(*parameters.StartTime)
	}

	if parameters.EndTime != nil && *parameters.EndTime != 0 {
		queryRequest.SetEndTime(*parameters.EndTime)
	}

	logEvents, err := logsClient.GetLogEventsWithContext(ctx, queryRequest)
	if err != nil {
		return nil, err
	}

	messages := make([]*string, 0)
	timestamps := make([]*int64, 0)

	sort.Slice(logEvents.Events, func(i, j int) bool {
		return *(logEvents.Events[i].Timestamp) > *(logEvents.Events[j].Timestamp)
	})

	for _, event := range logEvents.Events {
		messages = append(messages, event.Message)
		timestamps = append(timestamps, event.Timestamp)
	}

	timestampField := data.NewField("ts", nil, timestamps)
	timestampField.SetConfig(&data.FieldConfig{DisplayName: "Time"})

	messageField := data.NewField("line", nil, messages)

	return data.NewFrame("logEvents", timestampField, messageField), nil
}

func (e *cloudWatchExecutor) executeStartQuery(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson, timeRange backend.TimeRange) (*cloudwatchlogs.StartQueryOutput, error) {
	startTime := timeRange.From
	endTime := timeRange.To

	if !startTime.Before(endTime) {
		return nil, fmt.Errorf("invalid time range: start time must be before end time")
	}

	// The fields @log and @logStream are always included in the results of a user's query
	// so that a row's context can be retrieved later if necessary.
	// The usage of ltrim around the @log/@logStream fields is a necessary workaround, as without it,
	// CloudWatch wouldn't consider a query using a non-alised @log/@logStream valid.
	modifiedQueryString := "fields @timestamp,ltrim(@log) as " + logIdentifierInternal + ",ltrim(@logStream) as " +
		logStreamIdentifierInternal + "|" + parameters.QueryString

	startQueryInput := &cloudwatchlogs.StartQueryInput{
		StartTime: aws.Int64(startTime.Unix()),
		// Usually grafana time range allows only second precision, but you can create ranges with milliseconds
		// for example when going from trace to logs for that trace and trace length is sub second. In that case
		// StartTime is effectively floored while here EndTime is ceiled and so we should get the logs user wants
		// and also a little bit more but as CW logs accept only seconds as integers there is not much to do about
		// that.
		EndTime:     aws.Int64(int64(math.Ceil(float64(endTime.UnixNano()) / 1e9))),
		QueryString: aws.String(modifiedQueryString),
	}

	if e.features.IsEnabled(featuremgmt.FlagCloudWatchCrossAccountQuerying) {
		if parameters.LogGroups != nil && len(parameters.LogGroups) > 0 {
			var logGroupIdentifiers []string
			for _, lg := range parameters.LogGroups {
				arn := lg.Value
				// due to a bug in the startQuery api, we remove * from the arn, otherwise it throws an error
				logGroupIdentifiers = append(logGroupIdentifiers, strings.TrimSuffix(arn, "*"))
			}
			startQueryInput.LogGroupIdentifiers = aws.StringSlice(logGroupIdentifiers)
		}
	}

	if startQueryInput.LogGroupIdentifiers == nil {
		startQueryInput.LogGroupNames = aws.StringSlice(parameters.LogGroupNames)
	}

	if parameters.Limit != nil {
		startQueryInput.Limit = aws.Int64(*parameters.Limit)
	}

	logger.Debug("calling startquery with context with input", "input", startQueryInput)
	return logsClient.StartQueryWithContext(ctx, startQueryInput)
}

func (e *cloudWatchExecutor) handleStartQuery(ctx context.Context, logger log.Logger, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	model LogQueryJson, timeRange backend.TimeRange, refID string) (*data.Frame, error) {
	startQueryResponse, err := e.executeStartQuery(ctx, logsClient, model, timeRange)
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) && awsErr.Code() == "LimitExceededException" {
			logger.Debug("executeStartQuery limit exceeded", "err", awsErr)
			return nil, &AWSError{Code: limitExceededException, Message: err.Error()}
		}
		return nil, err
	}

	dataFrame := data.NewFrame(refID, data.NewField("queryId", nil, []string{*startQueryResponse.QueryId}))
	dataFrame.RefID = refID

	region := "default"
	if model.Region != "" {
		region = model.Region
	}

	dataFrame.Meta = &data.FrameMeta{
		Custom: map[string]interface{}{
			"Region": region,
		},
	}

	return dataFrame, nil
}

func (e *cloudWatchExecutor) executeStopQuery(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson) (*cloudwatchlogs.StopQueryOutput, error) {
	queryInput := &cloudwatchlogs.StopQueryInput{
		QueryId: aws.String(parameters.QueryId),
	}

	response, err := logsClient.StopQueryWithContext(ctx, queryInput)
	if err != nil {
		// If the query has already stopped by the time CloudWatch receives the stop query request,
		// an "InvalidParameterException" error is returned. For our purposes though the query has been
		// stopped, so we ignore the error.
		var awsErr awserr.Error
		if errors.As(err, &awsErr) && awsErr.Code() == "InvalidParameterException" {
			response = &cloudwatchlogs.StopQueryOutput{Success: aws.Bool(false)}
			err = nil
		}
	}

	return response, err
}

func (e *cloudWatchExecutor) handleStopQuery(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson) (*data.Frame, error) {
	response, err := e.executeStopQuery(ctx, logsClient, parameters)
	if err != nil {
		return nil, err
	}

	dataFrame := data.NewFrame("StopQueryResponse", data.NewField("success", nil, []bool{*response.Success}))
	return dataFrame, nil
}

func (e *cloudWatchExecutor) executeGetQueryResults(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	queryInput := &cloudwatchlogs.GetQueryResultsInput{
		QueryId: aws.String(parameters.QueryId),
	}

	return logsClient.GetQueryResultsWithContext(ctx, queryInput)
}

func (e *cloudWatchExecutor) handleGetQueryResults(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson, refID string) (*data.Frame, error) {
	getQueryResultsOutput, err := e.executeGetQueryResults(ctx, logsClient, parameters)
	if err != nil {
		return nil, err
	}

	dataFrame, err := logsResultsToDataframes(getQueryResultsOutput)
	if err != nil {
		return nil, err
	}

	dataFrame.Name = refID
	dataFrame.RefID = refID

	return dataFrame, nil
}

func (e *cloudWatchExecutor) handleGetLogGroupFields(ctx context.Context, logsClient cloudwatchlogsiface.CloudWatchLogsAPI,
	parameters LogQueryJson, refID string) (*data.Frame, error) {
	queryInput := &cloudwatchlogs.GetLogGroupFieldsInput{
		LogGroupName: aws.String(parameters.LogGroupName),
		Time:         aws.Int64(parameters.Time),
	}

	getLogGroupFieldsOutput, err := logsClient.GetLogGroupFieldsWithContext(ctx, queryInput)
	if err != nil {
		return nil, err
	}

	fieldNames := make([]*string, 0)
	fieldPercentages := make([]*int64, 0)

	for _, logGroupField := range getLogGroupFieldsOutput.LogGroupFields {
		fieldNames = append(fieldNames, logGroupField.Name)
		fieldPercentages = append(fieldPercentages, logGroupField.Percent)
	}

	dataFrame := data.NewFrame(
		refID,
		data.NewField("name", nil, fieldNames),
		data.NewField("percent", nil, fieldPercentages),
	)

	dataFrame.RefID = refID

	return dataFrame, nil
}
