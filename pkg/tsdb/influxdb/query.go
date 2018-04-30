package influxdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"regexp"

	"github.com/grafana/grafana/pkg/tsdb"
)

var (
	regexpOperatorPattern    = regexp.MustCompile(`^\/.*\/$`)
	regexpMeasurementPattern = regexp.MustCompile(`^\/.*\/$`)
)

func (query *Query) Build(queryContext *tsdb.TsdbQuery) (string, error) {
	var res string

	if query.UseRawQuery && query.RawQuery != "" {
		res = query.RawQuery
	} else {
		res = query.renderSelectors(queryContext)
		res += query.renderMeasurement()
		res += query.renderWhereClause()
		res += query.renderTimeFilter(queryContext)
		res += query.renderGroupBy(queryContext)
	}

	calculator := tsdb.NewIntervalCalculator(&tsdb.IntervalOptions{})
	interval := calculator.Calculate(queryContext.TimeRange, query.Interval)

	res = strings.Replace(res, "$timeFilter", query.renderTimeFilter(queryContext), -1)
	res = strings.Replace(res, "$interval", interval.Text, -1)
	res = strings.Replace(res, "$__interval_ms", strconv.FormatInt(interval.Value.Nanoseconds()/int64(time.Millisecond), 10), -1)
	res = strings.Replace(res, "$__interval", interval.Text, -1)
	return res, nil
}

func (query *Query) renderTags() []string {
	var res []string
	for i, tag := range query.Tags {
		str := ""

		if i > 0 {
			if tag.Condition == "" {
				str += "AND"
			} else {
				str += tag.Condition
			}
			str += " "
		}

		//If the operator is missing we fall back to sensible defaults
		if tag.Operator == "" {
			if regexpOperatorPattern.Match([]byte(tag.Value)) {
				tag.Operator = "=~"
			} else {
				tag.Operator = "="
			}
		}

		// quote value unless regex or number
		var textValue string
		if tag.Operator == "=~" || tag.Operator == "!~" {
			textValue = tag.Value
		} else if tag.Operator == "<" || tag.Operator == ">" {
			textValue = tag.Value
		} else {
			textValue = fmt.Sprintf("'%s'", strings.Replace(tag.Value, `\`, `\\`, -1))
		}

		res = append(res, fmt.Sprintf(`%s"%s" %s %s`, str, tag.Key, tag.Operator, textValue))
	}

	return res
}

func (query *Query) renderTimeFilter(queryContext *tsdb.TsdbQuery) string {
	from := "now() - " + queryContext.TimeRange.From
	to := ""

	if queryContext.TimeRange.To != "now" && queryContext.TimeRange.To != "" {
		to = " and time < now() - " + strings.Replace(queryContext.TimeRange.To, "now-", "", 1)
	}

	return fmt.Sprintf("time > %s%s", from, to)
}

func (query *Query) renderSelectors(queryContext *tsdb.TsdbQuery) string {
	res := "SELECT "

	var selectors []string
	for _, sel := range query.Selects {

		stk := ""
		for _, s := range *sel {
			stk = s.Render(query, queryContext, stk)
		}
		selectors = append(selectors, stk)
	}

	return res + strings.Join(selectors, ", ")
}

func (query *Query) renderMeasurement() string {
	var policy string
	if query.Policy == "" || query.Policy == "default" {
		policy = ""
	} else {
		policy = `"` + query.Policy + `".`
	}

	measurement := query.Measurement

	if !regexpMeasurementPattern.Match([]byte(measurement)) {
		measurement = fmt.Sprintf(`"%s"`, measurement)
	}

	return fmt.Sprintf(` FROM %s%s`, policy, measurement)
}

func (query *Query) renderWhereClause() string {
	res := " WHERE "
	conditions := query.renderTags()
	if len(conditions) > 0 {
		if len(conditions) > 1 {
			res += "(" + strings.Join(conditions, " ") + ")"
		} else {
			res += conditions[0]
		}
		res += " AND "
	}

	return res
}

func (query *Query) renderGroupBy(queryContext *tsdb.TsdbQuery) string {
	groupBy := ""
	for i, group := range query.GroupBy {
		if i == 0 {
			groupBy += " GROUP BY"
		}

		if i > 0 && group.Type != "fill" {
			groupBy += ", " //fill is so very special. fill is a creep, fill is a weirdo
		} else {
			groupBy += " "
		}

		groupBy += group.Render(query, queryContext, "")
	}

	return groupBy
}
