import { LogRowModel, toDataFrame, Field, FieldCache } from '@grafana/data';
import React, { useState, useEffect } from 'react';
import flatten from 'lodash/flatten';
import useAsync from 'react-use/lib/useAsync';

import { DataQueryResponse, DataQueryError } from '@grafana/data';

export interface LogRowContextRows {
  before?: string[];
  after?: string[];
}
export interface LogRowContextQueryErrors {
  before?: string;
  after?: string;
}

export interface HasMoreContextRows {
  before: boolean;
  after: boolean;
}

interface ResultType {
  data: string[][];
  errors: string[];
}

interface LogRowContextProviderProps {
  row: LogRowModel;
  getRowContext: (row: LogRowModel, options?: any) => Promise<DataQueryResponse>;
  children: (props: {
    result: LogRowContextRows;
    errors: LogRowContextQueryErrors;
    hasMoreContextRows: HasMoreContextRows;
    updateLimit: () => void;
  }) => JSX.Element;
}

export const getRowContexts = async (
  getRowContext: (row: LogRowModel, options?: any) => Promise<DataQueryResponse>,
  row: LogRowModel,
  limit: number
) => {
  const promises = [
    getRowContext(row, {
      limit,
    }),
    getRowContext(row, {
      // The start time is inclusive so we will get the one row we are using as context entry
      limit: limit + 1,
      direction: 'FORWARD',
    }),
  ];

  const results: Array<DataQueryResponse | DataQueryError> = await Promise.all(promises.map(p => p.catch(e => e)));

  return {
    data: results.map(result => {
      const dataResult: DataQueryResponse = result as DataQueryResponse;
      if (!dataResult.data) {
        return [];
      }

      const data: any[] = [];
      for (let index = 0; index < dataResult.data.length; index++) {
        const dataFrame = toDataFrame(dataResult.data[index]);
        const fieldCache = new FieldCache(dataFrame);
        const timestampField: Field<string> = fieldCache.getFieldByName('ts')!;
        const idField: Field<string> | undefined = fieldCache.getFieldByName('id');

        for (let fieldIndex = 0; fieldIndex < timestampField.values.length; fieldIndex++) {
          // TODO: this filtering is datasource dependant so it will make sense to move it there so the API is
          //  to return correct list of lines handling inclusive ranges or how to filter the correct line on the
          //  datasource.

          // Filter out the row that is the one used as a focal point for the context as we will get it in one of the
          // requests.
          if (idField) {
            // For Loki this means we filter only the one row. Issue is we could have other rows logged at the same
            // ns which came before but they come in the response that search for logs after. This means right now
            // we will show those as if they came after. This is not strictly correct but seems better than loosing them
            // and making this correct would mean quite a bit of complexity to shuffle things around and messing up
            //counts.
            if (idField.values.get(fieldIndex) === row.uid) {
              continue;
            }
          } else {
            // Fallback to timestamp. This should not happen right now as this feature is implemented only for loki
            // and that has ID. Later this branch could be used in other DS but mind that this could also filter out
            // logs which were logged in the same timestamp and that can be a problem depending on the precision.
            if (parseInt(timestampField.values.get(fieldIndex), 10) === row.timeEpochMs) {
              continue;
            }
          }

          const lineField: Field<string> = dataFrame.fields.filter(field => field.name === 'line')[0];
          const line = lineField.values.get(fieldIndex); // assuming that both fields have same length

          if (data.length === 0) {
            data[0] = [line];
          } else {
            data[0].push(line);
          }
        }
      }

      return data;
    }),
    errors: results.map(result => {
      const errorResult: DataQueryError = result as DataQueryError;
      if (!errorResult.message) {
        return '';
      }

      return errorResult.message;
    }),
  };
};

export const LogRowContextProvider: React.FunctionComponent<LogRowContextProviderProps> = ({
  getRowContext,
  row,
  children,
}) => {
  // React Hook that creates a number state value called limit to component state and a setter function called setLimit
  // The intial value for limit is 10
  // Used for the number of rows to retrieve from backend from a specific point in time
  const [limit, setLimit] = useState(10);

  // React Hook that creates an object state value called result to component state and a setter function called setResult
  // The intial value for result is null
  // Used for sorting the response from backend
  const [result, setResult] = useState<ResultType>((null as any) as ResultType);

  // React Hook that creates an object state value called hasMoreContextRows to component state and a setter function called setHasMoreContextRows
  // The intial value for hasMoreContextRows is {before: true, after: true}
  // Used for indicating in UI if there are more rows to load in a given direction
  const [hasMoreContextRows, setHasMoreContextRows] = useState({
    before: true,
    after: true,
  });

  // React Hook that resolves two promises every time the limit prop changes
  // First promise fetches limit number of rows backwards in time from a specific point in time
  // Second promise fetches limit number of rows forwards in time from a specific point in time
  const { value } = useAsync(async () => {
    return await getRowContexts(getRowContext, row, limit); // Moved it to a separate function for debugging purposes
  }, [limit]);

  // React Hook that performs a side effect every time the value (from useAsync hook) prop changes
  // The side effect changes the result state with the response from the useAsync hook
  // The side effect changes the hasMoreContextRows state if there are more context rows before or after the current result
  useEffect(() => {
    if (value) {
      setResult((currentResult: any) => {
        let hasMoreLogsBefore = true,
          hasMoreLogsAfter = true;

        if (currentResult && currentResult.data[0].length === value.data[0].length) {
          hasMoreLogsBefore = false;
        }

        if (currentResult && currentResult.data[1].length === value.data[1].length) {
          hasMoreLogsAfter = false;
        }

        setHasMoreContextRows({
          before: hasMoreLogsBefore,
          after: hasMoreLogsAfter,
        });

        return value;
      });
    }
  }, [value]);

  return children({
    result: {
      before: result ? flatten(result.data[0]) : [],
      after: result ? flatten(result.data[1]) : [],
    },
    errors: {
      before: result ? result.errors[0] : undefined,
      after: result ? result.errors[1] : undefined,
    },
    hasMoreContextRows,
    updateLimit: () => setLimit(limit + 10),
  });
};
