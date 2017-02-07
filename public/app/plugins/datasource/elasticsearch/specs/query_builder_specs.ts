
import {describe, beforeEach, it, sinon, expect} from 'test/lib/common';
import ElasticQueryBuilder from '../query_builder';

describe('ElasticQueryBuilder', function() {
  var builder;

  beforeEach(function() {
    builder = new ElasticQueryBuilder({timeField: '@timestamp'});
  });

  it('with defaults', function() {
    var query = builder.build({
      metrics: [{type: 'Count', id: '0'}],
      timeField: '@timestamp',
      bucketAggs: [{type: 'date_histogram', field: '@timestamp', id: '1'}],
    });

    expect(query.query.bool.must[0].range["@timestamp"].gte).to.be("$timeFrom");
    expect(query.aggs["1"].date_histogram.extended_bounds.min).to.be("$timeFrom");
  });

  it('with defaults on es5.x', function() {
    var builder_5x = new ElasticQueryBuilder({
      timeField: '@timestamp',
      esVersion: 5
    });

    var query = builder_5x.build({
      metrics: [{type: 'Count', id: '0'}],
      timeField: '@timestamp',
      bucketAggs: [{type: 'date_histogram', field: '@timestamp', id: '1'}],
    });

    expect(query.query.bool.must[0].range["@timestamp"].gte).to.be("$timeFrom");
    expect(query.aggs["1"].date_histogram.extended_bounds.min).to.be("$timeFrom");
  });

  it('with multiple bucket aggs', function() {
    var query = builder.build({
      metrics: [{type: 'count', id: '1'}],
      timeField: '@timestamp',
      bucketAggs: [
        {type: 'terms', field: '@host', id: '2'},
        {type: 'date_histogram', field: '@timestamp', id: '3'}
      ],
    });

    expect(query.aggs["2"].terms.field).to.be("@host");
    expect(query.aggs["2"].aggs["3"].date_histogram.field).to.be("@timestamp");
  });

  it('with select field', function() {
    var query = builder.build({
      metrics: [{type: 'avg', field: '@value', id: '1'}],
      bucketAggs: [{type: 'date_histogram', field: '@timestamp', id: '2'}],
    }, 100, 1000);

    var aggs = query.aggs["2"].aggs;
    expect(aggs["1"].avg.field).to.be("@value");
  });

  it('with term agg and order by metric agg', function() {
    var query = builder.build({
      metrics: [
        {type: 'count', id: '1'},
        {type: 'avg', field: '@value', id: '5'}
      ],
      bucketAggs: [
        {type: 'terms', field: '@host', settings: {size: 5, order: 'asc', orderBy: '5'}, id: '2' },
        {type: 'date_histogram', field: '@timestamp', id: '3'}
      ],
    }, 100, 1000);

    var firstLevel = query.aggs["2"];
    var secondLevel = firstLevel.aggs["3"];

    expect(firstLevel.aggs["5"].avg.field).to.be("@value");
    expect(secondLevel.aggs["5"].avg.field).to.be("@value");
  });

  it('with metric percentiles', function() {
    var query = builder.build({
      metrics: [
        {
          id: '1',
          type: 'percentiles',
          field: '@load_time',
          settings: {
            percents: [1,2,3,4]
          }
        }
      ],
      bucketAggs: [
        {type: 'date_histogram', field: '@timestamp', id: '3'}
      ],
    }, 100, 1000);

    var firstLevel = query.aggs["3"];

    expect(firstLevel.aggs["1"].percentiles.field).to.be("@load_time");
    expect(firstLevel.aggs["1"].percentiles.percents).to.eql([1,2,3,4]);
  });

  it('with filters aggs', function() {
    var query = builder.build({
      metrics: [{type: 'count', id: '1'}],
      timeField: '@timestamp',
      bucketAggs: [
        {
          id: '2',
          type: 'filters',
          settings: {
            filters: [
              {query: '@metric:cpu' },
              {query: '@metric:logins.count' },
            ]
          }
        },
        {type: 'date_histogram', field: '@timestamp', id: '4'}
      ],
    });

    expect(query.aggs["2"].filters.filters["@metric:cpu"].query_string.query).to.be("@metric:cpu");
    expect(query.aggs["2"].filters.filters["@metric:logins.count"].query_string.query).to.be("@metric:logins.count");
    expect(query.aggs["2"].aggs["4"].date_histogram.field).to.be("@timestamp");
  });

  it('with filters aggs on es5.x', function() {
    var builder_5x = new ElasticQueryBuilder({
      timeField: '@timestamp',
      esVersion: 5
    });
    var query = builder_5x.build({
      metrics: [{type: 'count', id: '1'}],
      timeField: '@timestamp',
      bucketAggs: [
        {
          id: '2',
          type: 'filters',
          settings: {
            filters: [
              {query: '@metric:cpu' },
              {query: '@metric:logins.count' },
            ]
          }
        },
        {type: 'date_histogram', field: '@timestamp', id: '4'}
      ],
    });

    expect(query.aggs["2"].filters.filters["@metric:cpu"].query_string.query).to.be("@metric:cpu");
    expect(query.aggs["2"].filters.filters["@metric:logins.count"].query_string.query).to.be("@metric:logins.count");
    expect(query.aggs["2"].aggs["4"].date_histogram.field).to.be("@timestamp");
  });

  it('with raw_document metric', function() {
    var query = builder.build({
      metrics: [{type: 'raw_document', id: '1'}],
      timeField: '@timestamp',
      bucketAggs: [],
    });

    expect(query.size).to.be(500);
  });

  it('with moving average', function() {
    var query = builder.build({
      metrics: [
        {
          id: '3',
          type: 'sum',
          field: '@value'
        },
        {
          id: '2',
          type: 'moving_avg',
          field: '3',
          pipelineAgg: '3'
        }
      ],
      bucketAggs: [
        {type: 'date_histogram', field: '@timestamp', id: '3'}
      ],
    });

    var firstLevel = query.aggs["3"];

    expect(firstLevel.aggs["2"]).not.to.be(undefined);
    expect(firstLevel.aggs["2"].moving_avg).not.to.be(undefined);
    expect(firstLevel.aggs["2"].moving_avg.buckets_path).to.be("3");
  });

  it('with broken moving average', function() {
      var query = builder.build({
          metrics: [
              {
                  id: '3',
                  type: 'sum',
                  field: '@value'
              },
              {
                  id: '2',
                  type: 'moving_avg',
                  pipelineAgg: '3'
              },
              {
                  id: '4',
                  type: 'moving_avg',
                  pipelineAgg: 'Metric to apply moving average'
              }
          ],
          bucketAggs: [
              { type: 'date_histogram', field: '@timestamp', id: '3' }
          ],
      });

      var firstLevel = query.aggs["3"];

      expect(firstLevel.aggs["2"]).not.to.be(undefined);
      expect(firstLevel.aggs["2"].moving_avg).not.to.be(undefined);
      expect(firstLevel.aggs["2"].moving_avg.buckets_path).to.be("3");
      expect(firstLevel.aggs["4"]).to.be(undefined);
  });

  it('with derivative', function() {
    var query = builder.build({
      metrics: [
        {
          id: '3',
          type: 'sum',
          field: '@value'
        },
        {
          id: '2',
          type: 'derivative',
          pipelineAgg: '3'
        }
      ],
      bucketAggs: [
        {type: 'date_histogram', field: '@timestamp', id: '3'}
      ],
    });

    var firstLevel = query.aggs["3"];

    expect(firstLevel.aggs["2"]).not.to.be(undefined);
    expect(firstLevel.aggs["2"].derivative).not.to.be(undefined);
    expect(firstLevel.aggs["2"].derivative.buckets_path).to.be("3");
  });

  it('with adhoc filters', function() {
    var query = builder.build({
      metrics: [{type: 'Count', id: '0'}],
      timeField: '@timestamp',
      bucketAggs: [{type: 'date_histogram', field: '@timestamp', id: '3'}],
    }, [
      {key: 'key1', operator: '=', value: 'value1'}
    ]);

    expect(query.query.bool.must[2].term["key1"]).to.be("value1");
  });
});
