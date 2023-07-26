import React, { FC, useState, useMemo } from "react";
import { TopQuery } from "../../../types";
import { getComparator, stableSort } from "../../CardinalityPanel/Table/helpers";
import { TopQueryPanelProps } from "../TopQueryPanel/TopQueryPanel";
import classNames from "classnames";
import { ArrowDropDownIcon, CopyIcon, PlayCircleOutlineIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { useSnack } from "../../../contexts/Snackbar";
import { Link } from "react-router-dom";
import router from "../../../router";

const TopQueryTable:FC<TopQueryPanelProps> = ({ rows, columns, defaultOrderBy }) => {
  const { showInfoMessage } = useSnack();

  const [orderBy, setOrderBy] = useState<keyof TopQuery>(defaultOrderBy || "count");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("desc");

  const sortedList = useMemo(() => stableSort(rows as [], getComparator(orderDir, orderBy)) as TopQuery[],
    [rows, orderBy, orderDir]);

  const onSortHandler = (key: keyof TopQuery) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const createSortHandler = (col: keyof TopQuery) => () => {
    onSortHandler(col);
  };

  const createCopyHandler = ({ query }: TopQuery) => () => {
    // TODO add useCopyToClipboard after merge https://github.com/VictoriaMetrics/VictoriaMetrics/pull/4145
    navigator.clipboard.writeText(query);
    showInfoMessage({ text: "Query has been copied", type: "success" });
  };

  return (
    <table className="vm-table">
      <thead className="vm-table-header">
        <tr className="vm-table__row vm-table__row_header">
          {columns.map((col) => (
            <th
              className="vm-table-cell vm-table-cell_header vm-table-cell_sort"
              onClick={createSortHandler(col.sortBy || col.key)}
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
          <th className="vm-table-cell vm-table-cell_header"/> {/* empty cell for actions */}
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
            <td className="vm-table-cell vm-table-cell_no-padding">
              <div className="vm-top-queries-panels__table-actions">
                <Tooltip title={"Execute query"}>
                  <Link
                    to={`${router.home}?g0.expr=${encodeURIComponent(row.query)}`}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <Button
                      variant="text"
                      size="small"
                      startIcon={<PlayCircleOutlineIcon/>}
                    />
                  </Link>
                </Tooltip>
                <Tooltip title={"Copy query"}>
                  <Button
                    variant="text"
                    size="small"
                    startIcon={<CopyIcon/>}
                    onClick={createCopyHandler(row)}
                  />
                </Tooltip>
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default TopQueryTable;
