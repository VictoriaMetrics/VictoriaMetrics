import React, { FC, useState, useMemo } from "react";
import { TopQuery } from "../../../types";
import { getComparator, stableSort } from "../../CardinalityPanel/Table/helpers";
import { TopQueryPanelProps } from "../TopQueryPanel/TopQueryPanel";
import classNames from "classnames";
import { ArrowDropDownIcon } from "../../../components/Main/Icons";

const TopQueryTable:FC<TopQueryPanelProps> = ({ rows, columns, defaultOrderBy }) => {

  const [orderBy, setOrderBy] = useState<keyof TopQuery>(defaultOrderBy || "count");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("desc");

  const sortedList = useMemo(() => stableSort(rows as [], getComparator(orderDir, orderBy)),
    [rows, orderBy, orderDir]);

  const onSortHandler = (key: keyof TopQuery) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const createSortHandler = (col: keyof TopQuery) => () => {
    onSortHandler(col);
  };

  return (
    <table className="vm-table">
      <thead className="vm-table-header">
        <tr className="vm-table__row vm-table__row_header">
          {columns.map((col) => (
            <th
              className="vm-table-cell vm-table-cell_header vm-table-cell_sort"
              onClick={createSortHandler(col.key)}
              key={col.key}
            >
              <div className="vm-table-cell__content">
                {col.title || col.key}
                <div
                  className={classNames({
                    "vm-table__sort-icon": true,
                    "vm-table__sort-icon_active": orderBy === col.key,
                    "vm-table__sort-icon_desc": orderDir === "desc" && orderBy === col.key
                  })}
                >
                  <ArrowDropDownIcon/>
                </div>
              </div>
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="vm-table-body">
        {sortedList.map((row, rowIndex) => (
          <tr
            className="vm-table__row"
            key={rowIndex}
          >
            {columns.map((col) => (
              <td
                className="vm-table-cell"
                key={col.key}
              >
                {row[col.key] || "-"}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default TopQueryTable;
