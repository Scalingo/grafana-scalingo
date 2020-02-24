import React, { useMemo } from 'react';
import { DataFrame } from '@grafana/data';
import { useSortBy, useTable, useBlockLayout, Cell } from 'react-table';
import { FixedSizeList } from 'react-window';
import { getColumns, getTableRows } from './utils';
import { useTheme } from '../../themes';
import { TableFilterActionCallback } from './types';
import { getTableStyles } from './styles';
import { TableCell } from './TableCell';

export interface Props {
  data: DataFrame;
  width: number;
  height: number;
  onCellClick?: TableFilterActionCallback;
}

export const Table = ({ data, height, onCellClick, width }: Props) => {
  const theme = useTheme();
  const tableStyles = getTableStyles(theme);

  const { getTableProps, headerGroups, rows, prepareRow } = useTable(
    {
      columns: useMemo(() => getColumns(data, width), [data]),
      data: useMemo(() => getTableRows(data), [data]),
    },
    useSortBy,
    useBlockLayout
  );

  const RenderRow = React.useCallback(
    ({ index, style }) => {
      const row = rows[index];
      prepareRow(row);
      return (
        <div {...row.getRowProps({ style })} className={tableStyles.row}>
          {row.cells.map((cell: Cell, index: number) => (
            <TableCell
              key={index}
              field={data.fields[cell.column.index]}
              tableStyles={tableStyles}
              cell={cell}
              onCellClick={onCellClick}
            />
          ))}
        </div>
      );
    },
    [prepareRow, rows]
  );

  return (
    <div {...getTableProps()} className={tableStyles.table}>
      <div>
        {headerGroups.map((headerGroup: any) => (
          <div className={tableStyles.thead} {...headerGroup.getHeaderGroupProps()}>
            {headerGroup.headers.map((column: any) => renderHeaderCell(column, tableStyles.headerCell))}
          </div>
        ))}
      </div>
      <FixedSizeList height={height} itemCount={rows.length} itemSize={tableStyles.rowHeight} width={width}>
        {RenderRow}
      </FixedSizeList>
    </div>
  );
};

function renderHeaderCell(column: any, className: string) {
  const headerProps = column.getHeaderProps(column.getSortByToggleProps());

  if (column.textAlign) {
    headerProps.style.textAlign = column.textAlign;
  }

  return (
    <div className={className} {...headerProps}>
      {column.render('Header')}
      <span>{column.isSorted ? (column.isSortedDesc ? ' 🔽' : ' 🔼') : ''}</span>
    </div>
  );
}
