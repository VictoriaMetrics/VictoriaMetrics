import React, { useState, useMemo } from "react";
import classNames from "classnames";
import { ArrowDropDownIcon, CopyIcon, DoneIcon } from "../Main/Icons";
import { getComparator, stableSort } from "./helpers";
import Tooltip from "../Main/Tooltip/Tooltip";
import Button from "../Main/Button/Button";
import { useEffect } from "preact/compat";

interface TableProps<T> {
  rows: T[];
  columns: { title?: string, key: keyof Partial<T>, className?: string }[];
  defaultOrderBy: keyof T;
  copyToClipboard?: keyof T;
  // TODO: Remove when pagination is implemented on the backend.
  paginationOffset: {
    startIndex: number;
    endIndex: number;
  }
}

const Table = <T extends object>({ rows, columns, defaultOrderBy, copyToClipboard, paginationOffset }: TableProps<T>) => {
  const [orderBy, setOrderBy] = useState<keyof T>(defaultOrderBy);
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("desc");
  const [copied, setCopied] = useState<number | null>(null);

  // const sortedList = useMemo(() => stableSort(rows as [], getComparator(orderDir, orderBy)),
  //   [rows, orderBy, orderDir]);
  // TODO: Remove when pagination is implemented on the backend.
  const sortedList = useMemo(() => {
    const { startIndex, endIndex } = paginationOffset;
    return stableSort(rows as [], getComparator(orderDir, orderBy)).slice(startIndex, endIndex);
  },
  [rows, orderBy, orderDir, paginationOffset]);

  const createSortHandler = (key: keyof T) => () => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const createCopyHandler = (copyValue:  string | number, rowIndex: number) => async () => {
    if (copied === rowIndex) return;
    try {
      await navigator.clipboard.writeText(String(copyValue));
      setCopied(rowIndex);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(null), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <table className="vm-table">
      <thead className="vm-table-header">
        <tr className="vm-table__row vm-table__row_header">
          {columns.map((col) => (
            <th
              className="vm-table-cell vm-table-cell_header vm-table-cell_sort"
              onClick={createSortHandler(col.key)}
              key={String(col.key)}
            >
              <div className="vm-table-cell__content">
                <div>
                  {String(col.title || col.key)}
                </div>
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
          {copyToClipboard && <th className="vm-table-cell vm-table-cell_header"/>}
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
                className={classNames({
                  "vm-table-cell": true,
                  [`${col.className}`]: col.className
                })}
                key={String(col.key)}
              >
                {row[col.key] || "-"}
              </td>
            ))}
            {copyToClipboard && (
              <td className="vm-table-cell vm-table-cell_right">
                {row[copyToClipboard] && (
                  <div className="vm-table-cell__content">
                    <Tooltip title={copied === rowIndex ? "Copied" : "Copy row"}>
                      <Button
                        variant="text"
                        color={copied === rowIndex ? "success" : "gray"}
                        size="small"
                        startIcon={copied === rowIndex ? <DoneIcon/> : <CopyIcon/>}
                        onClick={createCopyHandler(row[copyToClipboard], rowIndex)}
                        ariaLabel="copy row"
                      />
                    </Tooltip>
                  </div>
                )}
              </td>
            )}
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default Table;
