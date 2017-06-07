package mysql

import (
	"testing"

	"github.com/grafana/grafana/pkg/tsdb"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMacroEngine(t *testing.T) {
	Convey("MacroEngine", t, func() {

		Convey("interpolate __time function", func() {
			engine := &MySqlMacroEngine{}

			sql, err := engine.Interpolate("select $__time(time_column)")
			So(err, ShouldBeNil)

			So(sql, ShouldEqual, "select UNIX_TIMESTAMP(time_column) as time_sec")
		})

		Convey("interpolate __time function wrapped in aggregation", func() {
			engine := &MySqlMacroEngine{}

			sql, err := engine.Interpolate("select min($__time(time_column))")
			So(err, ShouldBeNil)

			So(sql, ShouldEqual, "select min(UNIX_TIMESTAMP(time_column) as time_sec)")
		})

		Convey("interpolate __timeFilter function", func() {
			engine := &MySqlMacroEngine{
				TimeRange: &tsdb.TimeRange{From: "5m", To: "now"},
			}

			sql, err := engine.Interpolate("WHERE $__timeFilter(time_column)")
			So(err, ShouldBeNil)

			So(sql, ShouldEqual, "WHERE time_column > FROM_UNIXTIME(18446744066914186738) AND time_column < FROM_UNIXTIME(18446744066914187038)")
		})

	})
}
