import { of } from 'rxjs';
import { BackendSrv, BackendSrvRequest, FetchResponse } from 'src/services';

import {
  DataSourceJsonData,
  DataQuery,
  DataSourceInstanceSettings,
  DataQueryRequest,
  DataQueryResponseData,
  MutableDataFrame,
  DataSourceRef,
} from '@grafana/data';

import { DataSourceWithBackend, standardStreamOptionsProvider, toStreamingDataResponse } from './DataSourceWithBackend';

class MyDataSource extends DataSourceWithBackend<DataQuery, DataSourceJsonData> {
  constructor(instanceSettings: DataSourceInstanceSettings<DataSourceJsonData>) {
    super(instanceSettings);
  }
}

const mockDatasourceRequest = jest.fn<Promise<FetchResponse>, BackendSrvRequest[]>();

const backendSrv = {
  fetch: (options: BackendSrvRequest) => {
    return of(mockDatasourceRequest(options));
  },
} as unknown as BackendSrv;

jest.mock('../services', () => ({
  ...jest.requireActual('../services'),
  getBackendSrv: () => backendSrv,
  getDataSourceSrv: () => {
    return {
      getInstanceSettings: (ref?: DataSourceRef) => ({
        type: ref?.type ?? '<mocktype>',
        uid: ref?.uid ?? '<mockuid>',
      }),
    };
  },
}));

describe('DataSourceWithBackend', () => {
  test('check the executed queries', () => {
    const { mock, ds } = createMockDatasource();
    ds.query({
      maxDataPoints: 10,
      intervalMs: 5000,
      targets: [{ refId: 'A' }, { refId: 'B', datasource: { type: 'sample' } }],
      dashboardUID: 'dashA',
      panelId: 123,
    } as DataQueryRequest);

    const args = mock.calls[0][0];

    expect(mock.calls.length).toBe(1);
    expect(args).toMatchInlineSnapshot(`
      Object {
        "data": Object {
          "queries": Array [
            Object {
              "datasource": Object {
                "type": "dummy",
                "uid": "abc",
              },
              "datasourceId": 1234,
              "intervalMs": 5000,
              "maxDataPoints": 10,
              "refId": "A",
            },
            Object {
              "datasource": Object {
                "type": "sample",
                "uid": "<mockuid>",
              },
              "datasourceId": undefined,
              "intervalMs": 5000,
              "maxDataPoints": 10,
              "refId": "B",
            },
          ],
        },
        "headers": Object {
          "X-Dashboard-Uid": "dashA",
          "X-Datasource-Uid": "abc, <mockuid>",
          "X-Panel-Id": "123",
          "X-Plugin-Id": "dummy, sample",
        },
        "hideFromInspector": false,
        "method": "POST",
        "requestId": undefined,
        "url": "/api/ds/query",
      }
    `);
  });

  test('should apply template variables only for the current data source', () => {
    const { mock, ds } = createMockDatasource();
    ds.applyTemplateVariables = jest.fn();
    ds.query({
      maxDataPoints: 10,
      intervalMs: 5000,
      targets: [{ refId: 'A' }, { refId: 'B', datasource: { type: 'sample' } }],
    } as DataQueryRequest);

    expect(mock.calls.length).toBe(1);
    expect(ds.applyTemplateVariables).toHaveBeenCalledTimes(1);
  });

  test('check that the executed queries is hidden from inspector', () => {
    const { mock, ds } = createMockDatasource();
    ds.query({
      maxDataPoints: 10,
      intervalMs: 5000,
      targets: [{ refId: 'A' }, { refId: 'B', datasource: { type: 'sample' } }],
      hideFromInspector: true,
      dashboardUID: 'dashA',
      panelId: 123,
    } as DataQueryRequest);

    const args = mock.calls[0][0];

    expect(mock.calls.length).toBe(1);
    expect(args).toMatchInlineSnapshot(`
      Object {
        "data": Object {
          "queries": Array [
            Object {
              "datasource": Object {
                "type": "dummy",
                "uid": "abc",
              },
              "datasourceId": 1234,
              "intervalMs": 5000,
              "maxDataPoints": 10,
              "refId": "A",
            },
            Object {
              "datasource": Object {
                "type": "sample",
                "uid": "<mockuid>",
              },
              "datasourceId": undefined,
              "intervalMs": 5000,
              "maxDataPoints": 10,
              "refId": "B",
            },
          ],
        },
        "headers": Object {
          "X-Dashboard-Uid": "dashA",
          "X-Datasource-Uid": "abc, <mockuid>",
          "X-Panel-Id": "123",
          "X-Plugin-Id": "dummy, sample",
        },
        "hideFromInspector": true,
        "method": "POST",
        "requestId": undefined,
        "url": "/api/ds/query",
      }
    `);
  });

  test('it converts results with channels to streaming queries', () => {
    const request: DataQueryRequest = {
      intervalMs: 100,
    } as DataQueryRequest;

    const rsp: DataQueryResponseData = {
      data: [],
    };

    // Simple empty query
    let obs = toStreamingDataResponse(rsp, request, standardStreamOptionsProvider);
    expect(obs).toBeDefined();

    let frame = new MutableDataFrame();
    frame.meta = {
      channel: 'a/b/c',
    };
    rsp.data = [frame];
    obs = toStreamingDataResponse(rsp, request, standardStreamOptionsProvider);
    expect(obs).toBeDefined();
  });
});

function createMockDatasource() {
  const settings = {
    name: 'test',
    id: 1234,
    uid: 'abc',
    type: 'dummy',
    jsonData: {},
  } as DataSourceInstanceSettings<DataSourceJsonData>;

  mockDatasourceRequest.mockReset();
  mockDatasourceRequest.mockReturnValue(Promise.resolve({} as FetchResponse));

  const ds = new MyDataSource(settings);
  return { ds, mock: mockDatasourceRequest.mock };
}
