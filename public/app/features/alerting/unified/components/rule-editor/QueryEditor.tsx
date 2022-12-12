import { css } from '@emotion/css';
import React, { FC } from 'react';

import { GrafanaTheme2, PanelData } from '@grafana/data';
import { useStyles2 } from '@grafana/ui';
import { AlertQuery } from 'app/types/unified-alerting-dto';

import { QueryRows } from './QueryRows';

interface Props {
  panelData: Record<string, PanelData>;
  queries: AlertQuery[];
  onRunQueries: () => void;
  onChangeQueries: (queries: AlertQuery[]) => void;
  onDuplicateQuery: (query: AlertQuery) => void;
  condition: string | null;
  onSetCondition: (refId: string) => void;
}

export const QueryEditor: FC<Props> = ({
  queries,
  panelData,
  onRunQueries,
  onChangeQueries,
  onDuplicateQuery,
  condition,
  onSetCondition,
}) => {
  const styles = useStyles2(getStyles);

  return (
    <div className={styles.container}>
      <QueryRows
        data={panelData}
        queries={queries}
        onRunQueries={onRunQueries}
        onQueriesChange={onChangeQueries}
        onDuplicateQuery={onDuplicateQuery}
        condition={condition}
        onSetCondition={onSetCondition}
      />
    </div>
  );
};

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    background-color: ${theme.colors.background.primary};
    height: 100%;
    max-width: ${theme.breakpoints.values.xxl}px;
  `,
});
