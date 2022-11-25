import React, { FC, useState } from "preact/compat";
import { ChangeEvent, MouseEvent } from "react";
import { Data, Order, TableProps, } from "./types";
import { EnhancedTableHead } from "./TableHead";
import { getComparator, stableSort } from "./helpers";
import classNames from "classnames";

const EnhancedTable: FC<TableProps> = ({
  rows,
  headerCells,
  defaultSortColumn,
  tableCells
}) => {

  const [order, setOrder] = useState<Order>("desc");
  const [orderBy, setOrderBy] = useState<keyof Data>(defaultSortColumn);
  const [selected, setSelected] = useState<readonly string[]>([]);

  const handleRequestSort = (
    event: MouseEvent<unknown>,
    property: keyof Data,
  ) => {
    const isAsc = orderBy === property && order === "asc";
    setOrder(isAsc ? "desc" : "asc");
    setOrderBy(property);
  };

  const handleSelectAllClick = (event: ChangeEvent<HTMLInputElement>) => {
    if (event.target.checked) {
      const newSelecteds = rows.map((n) => n.name) as string[];
      setSelected(newSelecteds);
      return;
    }
    setSelected([]);
  };

  const handleClick = (name: string) => () => {
    const selectedIndex = selected.indexOf(name);
    let newSelected: readonly string[] = [];

    if (selectedIndex === -1) {
      newSelected = newSelected.concat(selected, name);
    } else if (selectedIndex === 0) {
      newSelected = newSelected.concat(selected.slice(1));
    } else if (selectedIndex === selected.length - 1) {
      newSelected = newSelected.concat(selected.slice(0, -1));
    } else if (selectedIndex > 0) {
      newSelected = newSelected.concat(
        selected.slice(0, selectedIndex),
        selected.slice(selectedIndex + 1),
      );
    }

    setSelected(newSelected);
  };

  const isSelected = (name: string) => selected.indexOf(name) !== -1;
  const sortedData = stableSort(rows, getComparator(order, orderBy));

  return (
    <table className="vm-table">
      <EnhancedTableHead
        numSelected={selected.length}
        order={order}
        orderBy={orderBy}
        onSelectAllClick={handleSelectAllClick}
        onRequestSort={handleRequestSort}
        rowCount={rows.length}
        headerCells={headerCells}
      />
      <tbody className="vm-table-header">
        {sortedData.map((row) => (
          <tr
            className={classNames({
              "vm-table__row": true,
              "vm-table__row_selected": isSelected(row.name)
            })}
            key={row.name}
            onClick={handleClick(row.name)}
          >
            {tableCells(row)}
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default EnhancedTable;
