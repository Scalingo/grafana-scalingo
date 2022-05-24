import { css } from '@emotion/css';
import { capitalize } from 'lodash';
import React from 'react';

import { DisplayValue, formattedValueToString } from '@grafana/data';

import { useStyles } from '../../themes/ThemeContext';
import { InlineList } from '../List/InlineList';

/**
 * @internal
 */
export const VizLegendStatsList: React.FunctionComponent<{ stats: DisplayValue[] }> = ({ stats }) => {
  const styles = useStyles(getStyles);

  if (stats.length === 0) {
    return null;
  }

  return (
    <InlineList
      className={styles.list}
      items={stats}
      renderItem={(stat) => (
        <div className={styles.item} title={stat.description}>
          {stat.title && `${capitalize(stat.title)}:`} {formattedValueToString(stat)}
        </div>
      )}
    />
  );
};

const getStyles = () => ({
  list: css`
    flex-grow: 1;
    text-align: right;
  `,
  item: css`
    margin-left: 8px;
  `,
});

VizLegendStatsList.displayName = 'VizLegendStatsList';
