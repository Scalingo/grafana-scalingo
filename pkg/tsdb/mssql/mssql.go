package mssql

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/go-xorm/core"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
)

func init() {
	tsdb.RegisterTsdbQueryEndpoint("mssql", newMssqlQueryEndpoint)
}

func newMssqlQueryEndpoint(datasource *models.DataSource) (tsdb.TsdbQueryEndpoint, error) {
	logger := log.New("tsdb.mssql")

	cnnstr := generateConnectionString(datasource)
	logger.Debug("getEngine", "connection", cnnstr)

	config := tsdb.SqlQueryEndpointConfiguration{
		DriverName:        "mssql",
		ConnectionString:  cnnstr,
		Datasource:        datasource,
		MetricColumnTypes: []string{"VARCHAR", "CHAR", "NVARCHAR", "NCHAR"},
	}

	rowTransformer := mssqlRowTransformer{
		log: logger,
	}

	return tsdb.NewSqlQueryEndpoint(&config, &rowTransformer, newMssqlMacroEngine(), logger)
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
	encrypt := datasource.JsonData.Get("encrypt").MustString("false")
	connStr := fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;",
		server,
		port,
		datasource.Database,
		datasource.User,
		password,
	)
	if encrypt != "false" {
		connStr += fmt.Sprintf("encrypt=%s;", encrypt)
	}
	return connStr
}

type mssqlRowTransformer struct {
	log log.Logger
}

func (t *mssqlRowTransformer) Transform(columnTypes []*sql.ColumnType, rows *core.Rows) (tsdb.RowValues, error) {
	values := make([]interface{}, len(columnTypes))
	valuePtrs := make([]interface{}, len(columnTypes))

	for i, stype := range columnTypes {
		t.log.Debug("type", "type", stype)
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// convert types not handled by denisenkom/go-mssqldb
	// unhandled types are returned as []byte
	for i := 0; i < len(columnTypes); i++ {
		if value, ok := values[i].([]byte); ok {
			switch columnTypes[i].DatabaseTypeName() {
			case "MONEY", "SMALLMONEY", "DECIMAL":
				if v, err := strconv.ParseFloat(string(value), 64); err == nil {
					values[i] = v
				} else {
					t.log.Debug("Rows", "Error converting numeric to float", value)
				}
			default:
				t.log.Debug("Rows", "Unknown database type", columnTypes[i].DatabaseTypeName(), "value", value)
				values[i] = string(value)
			}
		}
	}

	return values, nil
}
