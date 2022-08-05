import { lastValueFrom } from 'rxjs';

import { ArrayVector, DataFrame, DataFrameView, getDisplayProcessor, SelectableValue } from '@grafana/data';
import { config, getDataSourceSrv } from '@grafana/runtime';
import { TermCount } from 'app/core/components/TagFilter/TagFilter';
import { GrafanaDatasource } from 'app/plugins/datasource/grafana/datasource';
import { GrafanaQueryType } from 'app/plugins/datasource/grafana/types';

import { DashboardQueryResult, GrafanaSearcher, QueryResponse, SearchQuery, SearchResultMeta } from '.';

export class BlugeSearcher implements GrafanaSearcher {
  async search(query: SearchQuery): Promise<QueryResponse> {
    if (query.facet?.length) {
      throw 'facets not supported!';
    }
    return doSearchQuery(query);
  }

  async tags(query: SearchQuery): Promise<TermCount[]> {
    const ds = (await getDataSourceSrv().get('-- Grafana --')) as GrafanaDatasource;
    const target = {
      refId: 'TagsQuery',
      queryType: GrafanaQueryType.Search,
      search: {
        ...query,
        query: query.query ?? '*',
        sort: undefined, // no need to sort the initial query results (not used)
        facet: [{ field: 'tag' }],
        limit: 1, // 0 would be better, but is ignored by the backend
      },
    };

    const data = (
      await lastValueFrom(
        ds.query({
          targets: [target],
        } as any)
      )
    ).data as DataFrame[];
    for (const frame of data) {
      if (frame.fields[0].name === 'tag') {
        return getTermCountsFrom(frame);
      }
    }
    return [];
  }

  // This should eventually be filled by an API call, but hardcoded is a good start
  getSortOptions(): Promise<SelectableValue[]> {
    const opts: SelectableValue[] = [
      { value: 'name_sort', label: 'Alphabetically (A-Z)' },
      { value: '-name_sort', label: 'Alphabetically (Z-A)' },
    ];

    if (config.licenseInfo.enabledFeatures.analytics) {
      for (const sf of sortFields) {
        opts.push({ value: `-${sf.name}`, label: `${sf.display} (most)` });
        opts.push({ value: `${sf.name}`, label: `${sf.display} (least)` });
      }
    }

    return Promise.resolve(opts);
  }
}

const firstPageSize = 50;
const nextPageSizes = 100;

async function doSearchQuery(query: SearchQuery): Promise<QueryResponse> {
  const ds = (await getDataSourceSrv().get('-- Grafana --')) as GrafanaDatasource;
  const target = {
    refId: 'Search',
    queryType: GrafanaQueryType.Search,
    search: {
      ...query,
      query: query.query ?? '*',
      limit: firstPageSize,
    },
  };
  const rsp = await lastValueFrom(
    ds.query({
      targets: [target],
    } as any)
  );

  const first = (rsp.data?.[0] as DataFrame) ?? { fields: [], length: 0 };
  for (const field of first.fields) {
    field.display = getDisplayProcessor({ field, theme: config.theme2 });
  }

  // Make sure the object exists
  if (!first.meta?.custom) {
    first.meta = {
      ...first.meta,
      custom: {
        count: first.length,
        max_score: 1,
      },
    };
  }

  const meta = first.meta.custom as SearchResultMeta;
  if (!meta.locationInfo) {
    meta.locationInfo = {}; // always set it so we can append
  }

  // Set the field name to a better display name
  if (meta.sortBy?.length) {
    const field = first.fields.find((f) => f.name === meta.sortBy);
    if (field) {
      const name = getSortFieldDisplayName(field.name);
      meta.sortBy = name;
      field.name = name; // make it look nicer
    }
  }

  const view = new DataFrameView<DashboardQueryResult>(first);
  return {
    totalRows: meta.count ?? first.length,
    view,
    loadMoreItems: async (startIndex: number, stopIndex: number): Promise<void> => {
      console.log('LOAD NEXT PAGE', { startIndex, stopIndex, length: view.dataFrame.length });
      const from = view.dataFrame.length;
      const limit = stopIndex - from;
      if (limit < 0) {
        return;
      }
      const frame = (
        await lastValueFrom(
          ds.query({
            targets: [
              {
                ...target,
                search: {
                  ...(target?.search ?? {}),
                  from,
                  limit: Math.max(limit, nextPageSizes),
                },
                refId: 'Page',
                facet: undefined,
              },
            ],
          } as any)
        )
      ).data?.[0] as DataFrame;

      if (!frame) {
        console.log('no results', frame);
        return;
      }
      if (frame.fields.length !== view.dataFrame.fields.length) {
        console.log('invalid shape', frame, view.dataFrame);
        return;
      }

      // Append the raw values to the same array buffer
      const length = frame.length + view.dataFrame.length;
      for (let i = 0; i < frame.fields.length; i++) {
        const values = (view.dataFrame.fields[i].values as ArrayVector).buffer;
        values.push(...frame.fields[i].values.toArray());
      }
      view.dataFrame.length = length;

      // Add all the location lookup info
      const submeta = frame.meta?.custom as SearchResultMeta;
      if (submeta?.locationInfo && meta) {
        for (const [key, value] of Object.entries(submeta.locationInfo)) {
          meta.locationInfo[key] = value;
        }
      }
      return;
    },
    isItemLoaded: (index: number): boolean => {
      return index < view.dataFrame.length;
    },
  };
}

function getTermCountsFrom(frame: DataFrame): TermCount[] {
  const keys = frame.fields[0].values;
  const vals = frame.fields[1].values;
  const counts: TermCount[] = [];
  for (let i = 0; i < frame.length; i++) {
    counts.push({ term: keys.get(i), count: vals.get(i) });
  }
  return counts;
}

// Enterprise only sort field values for dashboards
const sortFields = [
  { name: 'views_total', display: 'Views total' },
  { name: 'views_last_30_days', display: 'Views 30 days' },
  { name: 'errors_total', display: 'Errors total' },
  { name: 'errors_last_30_days', display: 'Errors 30 days' },
];

/** Given the internal field name, this gives a reasonable display name for the table colum header */
function getSortFieldDisplayName(name: string) {
  for (const sf of sortFields) {
    if (sf.name === name) {
      return sf.display;
    }
  }
  return name;
}
