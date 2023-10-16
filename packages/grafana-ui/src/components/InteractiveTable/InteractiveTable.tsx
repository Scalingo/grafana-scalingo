import { cx, css } from '@emotion/css';
import { uniqueId } from 'lodash';
import React, { useMemo, Fragment, ReactNode, useCallback } from 'react';
import { useExpanded, useSortBy, useTable, TableOptions, Row, HeaderGroup } from 'react-table';

import { GrafanaTheme2, isTruthy } from '@grafana/data';

import { useStyles2 } from '../../themes';
import { Icon } from '../Icon/Icon';

import { Column } from './types';
import { EXPANDER_CELL_ID, getColumns } from './utils';

const getStyles = (theme: GrafanaTheme2) => ({
  table: css`
    border-radius: ${theme.shape.borderRadius()};
    border: solid 1px ${theme.colors.border.weak};
    background-color: ${theme.colors.background.secondary};
    width: 100%;

    td {
      padding: ${theme.spacing(1)};
    }

    td,
    th {
      min-width: ${theme.spacing(3)};
    }
  `,
  evenRow: css`
    background: ${theme.colors.background.primary};
  `,
  disableGrow: css`
    width: 0%;
  `,
  header: css`
    &,
    & > button {
      position: relative;
      white-space: nowrap;
      padding: ${theme.spacing(1)};
    }
    & > button {
      &:after {
        content: '\\00a0';
      }
      width: 100%;
      height: 100%;
      background: none;
      border: none;
      padding-right: ${theme.spacing(2.5)};
      text-align: left;
      &:hover {
        background-color: ${theme.colors.emphasize(theme.colors.background.secondary, 0.05)};
      }
    }
  `,
  sortableHeader: css`
    /* increases selector's specificity so that it always takes precedence over default styles  */
    && {
      padding: 0;
    }
  `,
});

interface Props<TableData extends object> {
  /**
   * Table's columns definition. Must be memoized.
   */
  columns: Array<Column<TableData>>;
  /**
   * The data to display in the table. Must be memoized.
   */
  data: TableData[];
  /**
   * Render function for the expanded row. if not provided, the tables rows will not be expandable.
   */
  renderExpandedRow?: (row: TableData) => ReactNode;
  className?: string;
  /**
   * Must return a unique id for each row
   */
  getRowId: TableOptions<TableData>['getRowId'];
}

/** @alpha */
export function InteractiveTable<TableData extends object>({
  data,
  className,
  columns,
  renderExpandedRow,
  getRowId,
}: Props<TableData>) {
  const styles = useStyles2(getStyles);
  const tableColumns = useMemo(() => {
    const cols = getColumns<TableData>(columns);
    return cols;
  }, [columns]);
  const id = useUniqueId();
  const getRowHTMLID = useCallback(
    (row: Row<TableData>) => {
      return `${id}-${row.id}`.replace(/\s/g, '');
    },
    [id]
  );

  const { getTableProps, getTableBodyProps, headerGroups, rows, prepareRow } = useTable<TableData>(
    {
      columns: tableColumns,
      data,
      autoResetExpanded: false,
      autoResetSortBy: false,
      disableMultiSort: true,
      getRowId,
      initialState: {
        hiddenColumns: [
          !renderExpandedRow && EXPANDER_CELL_ID,
          ...tableColumns
            .filter((col) => !(col.visible ? col.visible(data) : true))
            .map((c) => c.id)
            .filter(isTruthy),
        ].filter(isTruthy),
      },
    },
    useSortBy,
    useExpanded
  );

  // This should be called only for rows thar we'd want to actually render, which is all at this stage.
  // We may want to revisit this if we decide to add pagination and/or virtualized tables.
  rows.forEach(prepareRow);

  return (
    <table {...getTableProps()} className={cx(styles.table, className)}>
      <thead>
        {headerGroups.map((headerGroup) => {
          const { key, ...headerRowProps } = headerGroup.getHeaderGroupProps();

          return (
            <tr key={key} {...headerRowProps}>
              {headerGroup.headers.map((column) => {
                const { key, ...headerCellProps } = column.getHeaderProps();

                return (
                  <th
                    key={key}
                    className={cx(styles.header, {
                      [styles.disableGrow]: column.width === 0,
                      [styles.sortableHeader]: column.canSort,
                    })}
                    {...headerCellProps}
                    {...(column.isSorted && { 'aria-sort': column.isSortedDesc ? 'descending' : 'ascending' })}
                  >
                    <ColumnHeader column={column} />
                  </th>
                );
              })}
            </tr>
          );
        })}
      </thead>

      <tbody {...getTableBodyProps()}>
        {rows.map((row, rowIndex) => {
          const className = cx(rowIndex % 2 === 0 && styles.evenRow);
          const { key, ...otherRowProps } = row.getRowProps();
          const rowId = getRowHTMLID(row);

          return (
            <Fragment key={key}>
              <tr className={className} {...otherRowProps}>
                {row.cells.map((cell) => {
                  const { key, ...otherCellProps } = cell.getCellProps();
                  return (
                    <td key={key} {...otherCellProps}>
                      {cell.render('Cell', { __rowID: rowId })}
                    </td>
                  );
                })}
              </tr>
              {
                // @ts-expect-error react-table doesn't ship with useExpanded types and we can't use declaration merging without affecting the table viz
                row.isExpanded && renderExpandedRow && (
                  <tr className={className} {...otherRowProps} id={rowId}>
                    <td colSpan={row.cells.length}>{renderExpandedRow(row.original)}</td>
                  </tr>
                )
              }
            </Fragment>
          );
        })}
      </tbody>
    </table>
  );
}

const useUniqueId = () => {
  return useMemo(() => uniqueId('InteractiveTable'), []);
};

const getColumnheaderStyles = (theme: GrafanaTheme2) => ({
  sortIcon: css`
    position: absolute;
    top: ${theme.spacing(1)};
  `,
});

function ColumnHeader<T extends object>({
  column: { canSort, render, isSorted, isSortedDesc, getSortByToggleProps },
}: {
  column: HeaderGroup<T>;
}) {
  const styles = useStyles2(getColumnheaderStyles);
  const { onClick } = getSortByToggleProps();

  const children = (
    <>
      {render('Header')}

      {isSorted && (
        <span aria-hidden="true" className={styles.sortIcon}>
          <Icon name={isSortedDesc ? 'angle-down' : 'angle-up'} />
        </span>
      )}
    </>
  );

  if (canSort) {
    return (
      <button type="button" onClick={onClick}>
        {children}
      </button>
    );
  }

  return children;
}
