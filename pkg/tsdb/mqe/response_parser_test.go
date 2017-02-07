package mqe

import (
	"testing"

	"net/http"
	"strings"

	"io/ioutil"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	dummieJson string
)

func TestMQEResponseParser(t *testing.T) {
	Convey("MQE response parser", t, func() {
		parser := NewResponseParser()

		Convey("Can parse response", func() {
			queryRef := &Query{
				AddClusterToAlias: true,
				AddHostToAlias:    true,
			}

			response := &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(dummieJson)),
			}
			res, err := parser.Parse(response, queryRef)
			So(err, ShouldBeNil)
			So(len(res), ShouldEqual, 2)
			So(len(res[0].Points), ShouldEqual, 14)
			So(res[0].Name, ShouldEqual, "demoapp staples-lab-1 os.disk.sda3.weighted_io_time")
			startTime := 1479287280000
			for i := 0; i < 11; i++ {
				So(res[0].Points[i][0].Float64, ShouldEqual, i+1)
				So(res[0].Points[i][1].Float64, ShouldEqual, startTime+(i*30000))
			}
		})
	})
}

func init() {
	dummieJson = `{
    "success": true,
    "name": "select",
    "body": [
      {
        "query": "os.disk.sda3.weighted_io_time",
        "name": "os.disk.sda3.weighted_io_time",
        "type": "series",
        "series": [
          {
            "tagset": {
              "cluster": "demoapp",
              "host": "staples-lab-1"
            },
            "values": [1,2,3,4,5,6,7,8,9,10,11, null, null, null]
          },
          {
            "tagset": {
              "cluster": "demoapp",
              "host": "staples-lab-2"
            },
            "values": [11,10,9,8,7,6,5,4,3,2,1]
          }
        ],
        "timerange": {
          "start": 1479287280000,
          "end": 1479287580000,
          "resolution": 30000
        }
      }
    ],
    "metadata": {
      "description": {
        "cluster": [
          "demoapp"
        ],
        "host": [
          "staples-lab-1",
          "staples-lab-2"
        ]
      },
      "notes": null,
      "profile": [
        {
          "name": "Parsing Query",
          "start": "2016-11-16T04:16:21.874354721-05:00",
          "finish": "2016-11-16T04:16:21.874762291-05:00"
        },
        {
          "name": "Cassandra GetAllTags",
          "start": "2016-11-16T04:16:21.874907171-05:00",
          "finish": "2016-11-16T04:16:21.876401922-05:00"
        },
        {
          "name": "CachedMetricMetadataAPI_GetAllTags_Expired",
          "start": "2016-11-16T04:16:21.874904751-05:00",
          "finish": "2016-11-16T04:16:21.876407852-05:00"
        },
        {
          "name": "CachedMetricMetadataAPI_GetAllTags",
          "start": "2016-11-16T04:16:21.874899491-05:00",
          "finish": "2016-11-16T04:16:21.876410382-05:00"
        },
        {
          "name": "Blueflood FetchSingleTimeseries Resolution",
          "description": "os.disk.sda3.weighted_io_time [app=demoapp,host=staples-lab-1]\n at 30s",
          "start": "2016-11-16T04:16:21.876623312-05:00",
          "finish": "2016-11-16T04:16:21.881763444-05:00"
        },
        {
          "name": "Blueflood FetchSingleTimeseries Resolution",
          "description": "os.disk.sda3.weighted_io_time [app=demoapp,host=staples-lab-2]\n at 30s",
          "start": "2016-11-16T04:16:21.876642682-05:00",
          "finish": "2016-11-16T04:16:21.881895914-05:00"
        },
        {
          "name": "Blueflood FetchMultipleTimeseries",
          "start": "2016-11-16T04:16:21.876418022-05:00",
          "finish": "2016-11-16T04:16:21.881921474-05:00"
        }
      ]
    }
  }
  `
}
