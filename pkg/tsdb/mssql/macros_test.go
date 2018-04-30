package mssql

import (
	"fmt"
	"strconv"
	"testing"

	"time"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/tsdb"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMacroEngine(t *testing.T) {
	Convey("MacroEngine", t, func() {
		engine := &MsSqlMacroEngine{}
		query := &tsdb.Query{
			Model: simplejson.New(),
		}

		Convey("Given a time range between 2018-04-12 00:00 and 2018-04-12 00:05", func() {
			from := time.Date(2018, 4, 12, 18, 0, 0, 0, time.UTC)
			to := from.Add(5 * time.Minute)
			timeRange := tsdb.NewFakeTimeRange("5m", "now", to)

			Convey("interpolate __time function", func() {
				sql, err := engine.Interpolate(query, nil, "select $__time(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, "select time_column AS time")
			})

			Convey("interpolate __timeEpoch function", func() {
				sql, err := engine.Interpolate(query, nil, "select $__timeEpoch(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, "select DATEDIFF(second, '1970-01-01', time_column) AS time")
			})

			Convey("interpolate __timeEpoch function wrapped in aggregation", func() {
				sql, err := engine.Interpolate(query, nil, "select min($__timeEpoch(time_column))")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, "select min(DATEDIFF(second, '1970-01-01', time_column) AS time)")
			})

			Convey("interpolate __timeFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "WHERE $__timeFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("WHERE time_column >= DATEADD(s, %d, '1970-01-01') AND time_column <= DATEADD(s, %d, '1970-01-01')", from.Unix(), to.Unix()))
			})

			Convey("interpolate __timeGroup function", func() {
				sql, err := engine.Interpolate(query, timeRange, "GROUP BY $__timeGroup(time_column,'5m')")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, "GROUP BY CAST(ROUND(DATEDIFF(second, '1970-01-01', time_column)/300.0, 0) as bigint)*300")
			})

			Convey("interpolate __timeGroup function with spaces around arguments", func() {
				sql, err := engine.Interpolate(query, timeRange, "GROUP BY $__timeGroup(time_column , '5m')")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, "GROUP BY CAST(ROUND(DATEDIFF(second, '1970-01-01', time_column)/300.0, 0) as bigint)*300")
			})

			Convey("interpolate __timeGroup function with fill (value = NULL)", func() {
				_, err := engine.Interpolate(query, timeRange, "GROUP BY $__timeGroup(time_column,'5m', NULL)")

				fill := query.Model.Get("fill").MustBool()
				fillNull := query.Model.Get("fillNull").MustBool()
				fillInterval := query.Model.Get("fillInterval").MustInt()

				So(err, ShouldBeNil)
				So(fill, ShouldBeTrue)
				So(fillNull, ShouldBeTrue)
				So(fillInterval, ShouldEqual, 5*time.Minute.Seconds())
			})

			Convey("interpolate __timeGroup function with fill (value = float)", func() {
				_, err := engine.Interpolate(query, timeRange, "GROUP BY $__timeGroup(time_column,'5m', 1.5)")

				fill := query.Model.Get("fill").MustBool()
				fillValue := query.Model.Get("fillValue").MustFloat64()
				fillInterval := query.Model.Get("fillInterval").MustInt()

				So(err, ShouldBeNil)
				So(fill, ShouldBeTrue)
				So(fillValue, ShouldEqual, 1.5)
				So(fillInterval, ShouldEqual, 5*time.Minute.Seconds())
			})

			Convey("interpolate __timeFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeFrom(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", from.Unix()))
			})

			Convey("interpolate __timeTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeTo(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", to.Unix()))
			})

			Convey("interpolate __unixEpochFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select time_column >= %d AND time_column <= %d", from.Unix(), to.Unix()))
			})

			Convey("interpolate __unixEpochFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFrom()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", from.Unix()))
			})

			Convey("interpolate __unixEpochTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochTo()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", to.Unix()))
			})
		})

		Convey("Given a time range between 1960-02-01 07:00 and 1965-02-03 08:00", func() {
			from := time.Date(1960, 2, 1, 7, 0, 0, 0, time.UTC)
			to := time.Date(1965, 2, 3, 8, 0, 0, 0, time.UTC)
			timeRange := tsdb.NewTimeRange(strconv.FormatInt(from.UnixNano()/int64(time.Millisecond), 10), strconv.FormatInt(to.UnixNano()/int64(time.Millisecond), 10))

			Convey("interpolate __timeFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "WHERE $__timeFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("WHERE time_column >= DATEADD(s, %d, '1970-01-01') AND time_column <= DATEADD(s, %d, '1970-01-01')", from.Unix(), to.Unix()))
			})

			Convey("interpolate __timeFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeFrom(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", from.Unix()))
			})

			Convey("interpolate __timeTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeTo(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", to.Unix()))
			})

			Convey("interpolate __unixEpochFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select time_column >= %d AND time_column <= %d", from.Unix(), to.Unix()))
			})

			Convey("interpolate __unixEpochFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFrom()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", from.Unix()))
			})

			Convey("interpolate __unixEpochTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochTo()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", to.Unix()))
			})
		})

		Convey("Given a time range between 1960-02-01 07:00 and 1980-02-03 08:00", func() {
			from := time.Date(1960, 2, 1, 7, 0, 0, 0, time.UTC)
			to := time.Date(1980, 2, 3, 8, 0, 0, 0, time.UTC)
			timeRange := tsdb.NewTimeRange(strconv.FormatInt(from.UnixNano()/int64(time.Millisecond), 10), strconv.FormatInt(to.UnixNano()/int64(time.Millisecond), 10))

			Convey("interpolate __timeFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "WHERE $__timeFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("WHERE time_column >= DATEADD(s, %d, '1970-01-01') AND time_column <= DATEADD(s, %d, '1970-01-01')", from.Unix(), to.Unix()))
			})

			Convey("interpolate __timeFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeFrom(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", from.Unix()))
			})

			Convey("interpolate __timeTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__timeTo(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select DATEADD(second, %d, '1970-01-01')", to.Unix()))
			})

			Convey("interpolate __unixEpochFilter function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFilter(time_column)")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select time_column >= %d AND time_column <= %d", from.Unix(), to.Unix()))
			})

			Convey("interpolate __unixEpochFrom function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochFrom()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", from.Unix()))
			})

			Convey("interpolate __unixEpochTo function", func() {
				sql, err := engine.Interpolate(query, timeRange, "select $__unixEpochTo()")
				So(err, ShouldBeNil)

				So(sql, ShouldEqual, fmt.Sprintf("select %d", to.Unix()))
			})
		})
	})
}
