import { render, screen } from '@testing-library/react';
import React from 'react';

import { addOperation } from 'app/plugins/datasource/prometheus/querybuilder/shared/OperationList.testUtils';

import { LokiDatasource } from '../../datasource';

import { LokiQueryBuilderContainer } from './LokiQueryBuilderContainer';

describe('LokiQueryBuilderContainer', () => {
  it('translates query between string and model', async () => {
    const props = {
      query: {
        expr: '{job="testjob"}',
        refId: 'A',
      },
      datasource: new LokiDatasource(
        {
          id: 1,
          uid: '',
          type: 'loki',
          name: 'loki-test',
          access: 'proxy',
          url: '',
          jsonData: {},
          meta: {} as any,
        },
        undefined,
        undefined
      ),
      onChange: jest.fn(),
      onRunQuery: () => {},
      showRawQuery: true,
    };
    render(<LokiQueryBuilderContainer {...props} />);
    expect(screen.getByText('testjob')).toBeInTheDocument();
    await addOperation('Range functions', 'Rate');
    expect(props.onChange).toBeCalledWith({
      expr: 'rate({job="testjob"} [$__interval])',
      refId: 'A',
    });
  });
});
