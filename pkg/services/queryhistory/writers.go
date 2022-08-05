package queryhistory

import (
	"bytes"
	"strings"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

func writeStarredSQL(query SearchInQueryHistoryQuery, sqlStore *sqlstore.SQLStore, builder *sqlstore.SQLBuilder) {
	if query.OnlyStarred {
		builder.Write(sqlStore.Dialect.BooleanStr(true) + ` AS starred
				FROM query_history
				INNER JOIN query_history_star ON query_history_star.query_uid = query_history.uid 
				`)
	} else {
		builder.Write(` CASE WHEN query_history_star.query_uid IS NULL THEN ` + sqlStore.Dialect.BooleanStr(false) + ` ELSE ` + sqlStore.Dialect.BooleanStr(true) + ` END AS starred
				FROM query_history
				LEFT JOIN query_history_star ON query_history_star.query_uid = query_history.uid 
				`)
	}
}

func writeFiltersSQL(query SearchInQueryHistoryQuery, user *models.SignedInUser, sqlStore *sqlstore.SQLStore, builder *sqlstore.SQLBuilder) {
	params := []interface{}{user.OrgId, user.UserId, query.From, query.To, "%" + query.SearchString + "%", "%" + query.SearchString + "%"}
	var sql bytes.Buffer
	sql.WriteString(" WHERE query_history.org_id = ? AND query_history.created_by = ? AND query_history.created_at >= ? AND query_history.created_at <= ? AND (query_history.queries " + sqlStore.Dialect.LikeStr() + " ? OR query_history.comment " + sqlStore.Dialect.LikeStr() + " ?) ")

	if len(query.DatasourceUIDs) > 0 {
		for _, uid := range query.DatasourceUIDs {
			params = append(params, uid)
		}
		sql.WriteString(" AND query_history.datasource_uid IN (? " + strings.Repeat(",?", len(query.DatasourceUIDs)-1) + ") ")
	}
	builder.Write(sql.String(), params...)
}

func writeSortSQL(query SearchInQueryHistoryQuery, sqlStore *sqlstore.SQLStore, builder *sqlstore.SQLBuilder) {
	if query.Sort == "time-asc" {
		builder.Write(" ORDER BY created_at ASC ")
	} else {
		builder.Write(" ORDER BY created_at DESC ")
	}
}

func writeLimitSQL(query SearchInQueryHistoryQuery, sqlStore *sqlstore.SQLStore, builder *sqlstore.SQLBuilder) {
	builder.Write(" LIMIT ? ", query.Limit)
}

func writeOffsetSQL(query SearchInQueryHistoryQuery, sqlStore *sqlstore.SQLStore, builder *sqlstore.SQLBuilder) {
	builder.Write(" OFFSET ? ", query.Limit*(query.Page-1))
}
