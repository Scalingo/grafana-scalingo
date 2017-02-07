package conditions

import (
	"testing"

	"gopkg.in/guregu/null.v3"

	"github.com/grafana/grafana/pkg/tsdb"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSimpleReducer(t *testing.T) {
	Convey("Test simple reducer by calculating", t, func() {

		Convey("sum", func() {
			result := testReducer("sum", 1, 2, 3)
			So(result, ShouldEqual, float64(6))
		})

		Convey("min", func() {
			result := testReducer("min", 3, 2, 1)
			So(result, ShouldEqual, float64(1))
		})

		Convey("max", func() {
			result := testReducer("max", 1, 2, 3)
			So(result, ShouldEqual, float64(3))
		})

		Convey("count", func() {
			result := testReducer("count", 1, 2, 3000)
			So(result, ShouldEqual, float64(3))
		})

		Convey("last", func() {
			result := testReducer("last", 1, 2, 3000)
			So(result, ShouldEqual, float64(3000))
		})

		Convey("median odd amount of numbers", func() {
			result := testReducer("median", 1, 2, 3000)
			So(result, ShouldEqual, float64(2))
		})

		Convey("median even amount of numbers", func() {
			result := testReducer("median", 1, 2, 4, 3000)
			So(result, ShouldEqual, float64(3))
		})

		Convey("median with one values", func() {
			result := testReducer("median", 1)
			So(result, ShouldEqual, float64(1))
		})

		Convey("avg", func() {
			result := testReducer("avg", 1, 2, 3)
			So(result, ShouldEqual, float64(2))
		})

		Convey("avg of number values and null values should ignore nulls", func() {
			reducer := NewSimpleReducer("avg")
			series := &tsdb.TimeSeries{
				Name: "test time serie",
			}

			series.Points = append(series.Points, tsdb.NewTimePoint(null.FloatFrom(3), 1))
			series.Points = append(series.Points, tsdb.NewTimePoint(null.FloatFromPtr(nil), 2))
			series.Points = append(series.Points, tsdb.NewTimePoint(null.FloatFromPtr(nil), 3))
			series.Points = append(series.Points, tsdb.NewTimePoint(null.FloatFrom(3), 4))

			So(reducer.Reduce(series).Float64, ShouldEqual, float64(3))
		})
	})
}

func testReducer(typ string, datapoints ...float64) float64 {
	reducer := NewSimpleReducer(typ)
	series := &tsdb.TimeSeries{
		Name: "test time serie",
	}

	for idx := range datapoints {
		series.Points = append(series.Points, tsdb.NewTimePoint(null.FloatFrom(datapoints[idx]), 1234134))
	}

	return reducer.Reduce(series).Float64
}
