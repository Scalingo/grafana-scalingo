import React, { useMemo } from 'react';

import { DataSourceApi, SelectableValue } from '@grafana/data';
import { EditorRow } from '@grafana/experimental';
import { LabelFilters } from 'app/plugins/datasource/prometheus/querybuilder/shared/LabelFilters';
import { OperationList } from 'app/plugins/datasource/prometheus/querybuilder/shared/OperationList';
import { OperationsEditorRow } from 'app/plugins/datasource/prometheus/querybuilder/shared/OperationsEditorRow';
import { QueryBuilderLabelFilter } from 'app/plugins/datasource/prometheus/querybuilder/shared/types';

import { LokiDatasource } from '../../datasource';
import { escapeLabelValueInSelector } from '../../language_utils';
import { lokiQueryModeller } from '../LokiQueryModeller';
import { LokiOperationId, LokiVisualQuery } from '../types';

import { NestedQueryList } from './NestedQueryList';

export interface Props {
  query: LokiVisualQuery;
  datasource: LokiDatasource;
  onChange: (update: LokiVisualQuery) => void;
  onRunQuery: () => void;
  nested?: boolean;
}

export const LokiQueryBuilder = React.memo<Props>(({ datasource, query, nested, onChange, onRunQuery }) => {
  const onChangeLabels = (labels: QueryBuilderLabelFilter[]) => {
    onChange({ ...query, labels });
  };

  const withTemplateVariableOptions = async (optionsPromise: Promise<string[]>): Promise<SelectableValue[]> => {
    const options = await optionsPromise;
    return [...datasource.getVariables(), ...options].map((value) => ({ label: value, value }));
  };

  const onGetLabelNames = async (forLabel: Partial<QueryBuilderLabelFilter>): Promise<any> => {
    const labelsToConsider = query.labels.filter((x) => x !== forLabel);

    if (labelsToConsider.length === 0) {
      await datasource.languageProvider.refreshLogLabels();
      return datasource.languageProvider.getLabelKeys();
    }

    const expr = lokiQueryModeller.renderLabels(labelsToConsider);
    const series = await datasource.languageProvider.fetchSeriesLabels(expr);
    return Object.keys(series).sort();
  };

  const onGetLabelValues = async (forLabel: Partial<QueryBuilderLabelFilter>) => {
    if (!forLabel.label) {
      return [];
    }

    let values;
    const labelsToConsider = query.labels.filter((x) => x !== forLabel);
    if (labelsToConsider.length === 0) {
      values = await datasource.languageProvider.fetchLabelValues(forLabel.label);
    } else {
      const expr = lokiQueryModeller.renderLabels(labelsToConsider);
      const result = await datasource.languageProvider.fetchSeriesLabels(expr);
      values = result[datasource.interpolateString(forLabel.label)];
    }

    return values ? values.map((v) => escapeLabelValueInSelector(v, forLabel.op)) : []; // Escape values in return
  };

  const labelFilterError: string | undefined = useMemo(() => {
    const { labels, operations: op } = query;
    if (!labels.length && op.length) {
      // We don't want to show error for initial state with empty line contains operation
      if (op.length === 1 && op[0].id === LokiOperationId.LineContains && op[0].params[0] === '') {
        return undefined;
      }
      return 'You need to specify at least 1 label filter (stream selector)';
    }
    return undefined;
  }, [query]);

  return (
    <>
      <EditorRow>
        <LabelFilters
          onGetLabelNames={(forLabel: Partial<QueryBuilderLabelFilter>) =>
            withTemplateVariableOptions(onGetLabelNames(forLabel))
          }
          onGetLabelValues={(forLabel: Partial<QueryBuilderLabelFilter>) =>
            withTemplateVariableOptions(onGetLabelValues(forLabel))
          }
          labelsFilters={query.labels}
          onChange={onChangeLabels}
          error={labelFilterError}
        />
      </EditorRow>
      <OperationsEditorRow>
        <OperationList
          queryModeller={lokiQueryModeller}
          query={query}
          onChange={onChange}
          onRunQuery={onRunQuery}
          datasource={datasource as DataSourceApi}
        />
      </OperationsEditorRow>
      {query.binaryQueries && query.binaryQueries.length > 0 && (
        <NestedQueryList query={query} datasource={datasource} onChange={onChange} onRunQuery={onRunQuery} />
      )}
    </>
  );
});

LokiQueryBuilder.displayName = 'LokiQueryBuilder';
