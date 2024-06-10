import React, { FC, useState } from "preact/compat";
import { MouseEvent } from "react";
import { Data, Order, TableProps, } from "./types";
import { EnhancedTableHead } from "./TableHead";
import { getComparator, stableSort } from "../../../components/Table/helpers";

const EnhancedTable: FC<TableProps> = ({
  rows,
  headerCells,
  defaultSortColumn,
  tableCells
}) => {

  const [order, setOrder] = useState<Order>("desc");
  const [orderBy, setOrderBy] = useState<keyof Data>(defaultSortColumn);

  const handleRequestSort = (
    event: MouseEvent<unknown>,
    property: keyof Data,
  ) => {
    const isAsc = orderBy === property && order === "asc";
    setOrder(isAsc ? "desc" : "asc");
    setOrderBy(property);
  };

  const sortedData = stableSort(rows, getComparator(order, orderBy));

  return (
    <table className="vm-table vm-cardinality-panel-table">
      <EnhancedTableHead
        order={order}
        orderBy={orderBy}
        onRequestSort={handleRequestSort}
        rowCount={rows.length}
        headerCells={headerCells}
      />
      <tbody className="vm-table-header">
        {sortedData.map((row) => (
          <tr
            className="vm-table__row"
            key={row.name}
          >
            {tableCells(row)}
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default EnhancedTable;
