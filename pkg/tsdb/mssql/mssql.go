package mssql

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"math"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/go-xorm/core"
	"github.com/grafana/grafana/pkg/components/null"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
)

type MssqlQueryEndpoint struct {
	sqlEngine tsdb.SqlEngine
	log       log.Logger
}

func init() {
	tsdb.RegisterTsdbQueryEndpoint("mssql", NewMssqlQueryEndpoint)
}

func NewMssqlQueryEndpoint(datasource *models.DataSource) (tsdb.TsdbQueryEndpoint, error) {
	endpoint := &MssqlQueryEndpoint{
		log: log.New("tsdb.mssql"),
	}

	endpoint.sqlEngine = &tsdb.DefaultSqlEngine{
		MacroEngine: NewMssqlMacroEngine(),
	}

	cnnstr := generateConnectionString(datasource)
	endpoint.log.Debug("getEngine", "connection", cnnstr)

	if err := endpoint.sqlEngine.InitEngine("mssql", datasource, cnnstr); err != nil {
		return nil, err
	}

	return endpoint, nil
}

func generateConnectionString(datasource *models.DataSource) string {
	password := ""
	for key, value := range datasource.SecureJsonData.Decrypt() {
		if key == "password" {
			password = value
			break
		}
	}

	hostParts := strings.Split(datasource.Url, ":")
	if len(hostParts) < 2 {
		hostParts = append(hostParts, "1433")
	}

	server, port := hostParts[0], hostParts[1]
	return fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;",
		server,
		port,
		datasource.Database,
		datasource.User,
		password,
	)
}

// Query is the main function for the MssqlQueryEndpoint
func (e *MssqlQueryEndpoint) Query(ctx context.Context, dsInfo *models.DataSource, tsdbQuery *tsdb.TsdbQuery) (*tsdb.Response, error) {
	return e.sqlEngine.Query(ctx, dsInfo, tsdbQuery, e.transformToTimeSeries, e.transformToTable)
}

func (e MssqlQueryEndpoint) transformToTable(query *tsdb.Query, rows *core.Rows, result *tsdb.QueryResult, tsdbQuery *tsdb.TsdbQuery) error {
	columnNames, err := rows.Columns()
	columnCount := len(columnNames)

	if err != nil {
		return err
	}

	rowLimit := 1000000
	rowCount := 0
	timeIndex := -1

	table := &tsdb.Table{
		Columns: make([]tsdb.TableColumn, columnCount),
		Rows:    make([]tsdb.RowValues, 0),
	}

	for i, name := range columnNames {
		table.Columns[i].Text = name

		// check if there is a column named time
		switch name {
		case "time":
			timeIndex = i
		}
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	for ; rows.Next(); rowCount++ {
		if rowCount > rowLimit {
			return fmt.Errorf("MsSQL query row limit exceeded, limit %d", rowLimit)
		}

		values, err := e.getTypedRowData(columnTypes, rows)
		if err != nil {
			return err
		}

		// converts column named time to unix timestamp in milliseconds
		// to make native mssql datetime types and epoch dates work in
		// annotation and table queries.
		tsdb.ConvertSqlTimeColumnToEpochMs(values, timeIndex)
		table.Rows = append(table.Rows, values)
	}

	result.Tables = append(result.Tables, table)
	result.Meta.Set("rowCount", rowCount)
	return nil
}

func (e MssqlQueryEndpoint) getTypedRowData(types []*sql.ColumnType, rows *core.Rows) (tsdb.RowValues, error) {
	values := make([]interface{}, len(types))
	valuePtrs := make([]interface{}, len(types))

	for i, stype := range types {
		e.log.Debug("type", "type", stype)
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// convert types not handled by denisenkom/go-mssqldb
	// unhandled types are returned as []byte
	for i := 0; i < len(types); i++ {
		if value, ok := values[i].([]byte); ok {
			switch types[i].DatabaseTypeName() {
			case "MONEY", "SMALLMONEY", "DECIMAL":
				if v, err := strconv.ParseFloat(string(value), 64); err == nil {
					values[i] = v
				} else {
					e.log.Debug("Rows", "Error converting numeric to float", value)
				}
			default:
				e.log.Debug("Rows", "Unknown database type", types[i].DatabaseTypeName(), "value", value)
				values[i] = string(value)
			}
		}
	}

	return values, nil
}

func (e MssqlQueryEndpoint) transformToTimeSeries(query *tsdb.Query, rows *core.Rows, result *tsdb.QueryResult, tsdbQuery *tsdb.TsdbQuery) error {
	pointsBySeries := make(map[string]*tsdb.TimeSeries)
	seriesByQueryOrder := list.New()

	columnNames, err := rows.Columns()
	if err != nil {
		return err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	rowLimit := 1000000
	rowCount := 0
	timeIndex := -1
	metricIndex := -1

	// check columns of resultset: a column named time is mandatory
	// the first text column is treated as metric name unless a column named metric is present
	for i, col := range columnNames {
		switch col {
		case "time":
			timeIndex = i
		case "metric":
			metricIndex = i
		default:
			if metricIndex == -1 {
				switch columnTypes[i].DatabaseTypeName() {
				case "VARCHAR", "CHAR", "NVARCHAR", "NCHAR":
					metricIndex = i
				}
			}
		}
	}

	if timeIndex == -1 {
		return fmt.Errorf("Found no column named time")
	}

	fillMissing := query.Model.Get("fill").MustBool(false)
	var fillInterval float64
	fillValue := null.Float{}
	if fillMissing {
		fillInterval = query.Model.Get("fillInterval").MustFloat64() * 1000
		if !query.Model.Get("fillNull").MustBool(false) {
			fillValue.Float64 = query.Model.Get("fillValue").MustFloat64()
			fillValue.Valid = true
		}
	}

	for rows.Next() {
		var timestamp float64
		var value null.Float
		var metric string

		if rowCount > rowLimit {
			return fmt.Errorf("MSSQL query row limit exceeded, limit %d", rowLimit)
		}

		values, err := e.getTypedRowData(columnTypes, rows)
		if err != nil {
			return err
		}

		// converts column named time to unix timestamp in milliseconds to make
		// native mysql datetime types and epoch dates work in
		// annotation and table queries.
		tsdb.ConvertSqlTimeColumnToEpochMs(values, timeIndex)

		switch columnValue := values[timeIndex].(type) {
		case int64:
			timestamp = float64(columnValue)
		case float64:
			timestamp = columnValue
		default:
			return fmt.Errorf("Invalid type for column time, must be of type timestamp or unix timestamp, got: %T %v", columnValue, columnValue)
		}

		if metricIndex >= 0 {
			if columnValue, ok := values[metricIndex].(string); ok {
				metric = columnValue
			} else {
				return fmt.Errorf("Column metric must be of type CHAR, VARCHAR, NCHAR or NVARCHAR. metric column name: %s type: %s but datatype is %T", columnNames[metricIndex], columnTypes[metricIndex].DatabaseTypeName(), values[metricIndex])
			}
		}

		for i, col := range columnNames {
			if i == timeIndex || i == metricIndex {
				continue
			}

			if value, err = tsdb.ConvertSqlValueColumnToFloat(col, values[i]); err != nil {
				return err
			}

			if metricIndex == -1 {
				metric = col
			}

			series, exist := pointsBySeries[metric]
			if !exist {
				series = &tsdb.TimeSeries{Name: metric}
				pointsBySeries[metric] = series
				seriesByQueryOrder.PushBack(metric)
			}

			if fillMissing {
				var intervalStart float64
				if !exist {
					intervalStart = float64(tsdbQuery.TimeRange.MustGetFrom().UnixNano() / 1e6)
				} else {
					intervalStart = series.Points[len(series.Points)-1][1].Float64 + fillInterval
				}

				// align interval start
				intervalStart = math.Floor(intervalStart/fillInterval) * fillInterval

				for i := intervalStart; i < timestamp; i += fillInterval {
					series.Points = append(series.Points, tsdb.TimePoint{fillValue, null.FloatFrom(i)})
					rowCount++
				}
			}

			series.Points = append(series.Points, tsdb.TimePoint{value, null.FloatFrom(timestamp)})

			e.log.Debug("Rows", "metric", metric, "time", timestamp, "value", value)
		}
	}

	for elem := seriesByQueryOrder.Front(); elem != nil; elem = elem.Next() {
		key := elem.Value.(string)
		result.Series = append(result.Series, pointsBySeries[key])

		if fillMissing {
			series := pointsBySeries[key]
			// fill in values from last fetched value till interval end
			intervalStart := series.Points[len(series.Points)-1][1].Float64
			intervalEnd := float64(tsdbQuery.TimeRange.MustGetTo().UnixNano() / 1e6)

			// align interval start
			intervalStart = math.Floor(intervalStart/fillInterval) * fillInterval
			for i := intervalStart + fillInterval; i < intervalEnd; i += fillInterval {
				series.Points = append(series.Points, tsdb.TimePoint{fillValue, null.FloatFrom(i)})
				rowCount++
			}
		}
	}

	result.Meta.Set("rowCount", rowCount)
	return nil
}
