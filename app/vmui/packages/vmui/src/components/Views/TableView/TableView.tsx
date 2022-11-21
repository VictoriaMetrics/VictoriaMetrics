import React, { FC, useMemo, useState } from "preact/compat";
import { InstantMetricResult } from "../../../api/types";
import { InstantDataSeries } from "../../../types";
import { useSortedCategories } from "../../../hooks/useSortedCategories";
import Alert from "../../Main/Alert/Alert";
import classNames from "classnames";
import { ArrowDropDownIcon } from "../../Main/Icons";
import { getNameForMetric } from "../../../utils/metric";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";

export interface GraphViewProps {
  data: InstantMetricResult[];
  displayColumns?: string[]
}

const TableView: FC<GraphViewProps> = ({ data, displayColumns }) => {

  const { tableCompact } = useCustomPanelState();
  const [orderBy, setOrderBy] = useState("");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("asc");

  const sortedColumns = (tableCompact
    ? useSortedCategories([{ group: 0, metric: { "Data": "Data" } }], ["Data"])
    : useSortedCategories(data, displayColumns)
  );

  const rows: InstantDataSeries[] = useMemo(() => {
    const rows = data?.map(d => ({
      metadata: sortedColumns.map(c => (tableCompact
        ? getNameForMetric(d, undefined, "=", true)
        : (d.metric[c.key] || "-")
      )),
      value: d.value ? d.value[1] : "-"
    }));
    const orderByValue = orderBy === "Value";
    const rowIndex = sortedColumns.findIndex(c => c.key === orderBy);
    if (!orderByValue && rowIndex === -1) return rows;
    return rows.sort((a,b) => {
      const n1 = orderByValue ? Number(a.value) : a.metadata[rowIndex];
      const n2 = orderByValue ? Number(b.value) : b.metadata[rowIndex];
      const asc = orderDir === "asc" ? n1 < n2 : n1 > n2;
      return asc ? -1 : 1;
    });
  }, [sortedColumns, data, orderBy, orderDir, tableCompact]);

  const createSortHandler = (key: string) => () => {
    sortHandler(key);
  };

  const sortHandler = (key: string) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  if (!rows.length) return <Alert variant="warning">No data to show</Alert>;

  return (
    <table className="vm-table">
      <thead className="vm-table-header">
        <tr className="vm-table__row vm-table__row_header">
          {sortedColumns.map((col, index) => (
            <td
              className="vm-table-cell vm-table-cell_header vm-table-cell_sort"
              key={index}
              onClick={createSortHandler(col.key)}
            >
              <div className="vm-table-cell__content">
                {col.key}
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
            </td>
          ))}
          <td
            className="vm-table-cell vm-table-cell_header vm-table-cell_right vm-table-cell_sort"
            onClick={createSortHandler("Value")}
          >
            <div className="vm-table-cell__content">
              <div
                className={classNames({
                  "vm-table__sort-icon": true,
                  "vm-table__sort-icon_active": orderBy === "Value",
                  "vm-table__sort-icon_desc": orderDir === "desc"
                })}
              >
                <ArrowDropDownIcon/>
              </div>
            Value
            </div>
          </td>
        </tr>
      </thead>
      <tbody className="vm-table-body">
        {rows.map((row, index) => (
          <tr
            className="vm-table__row"
            key={index}
          >
            {row.metadata.map((rowMeta, index2) => (
              <td
                className={classNames({
                  "vm-table-cell vm-table-cell_no-wrap": true,
                  "vm-table-cell_gray":  rows[index - 1] && rows[index - 1].metadata[index2] === rowMeta
                })}
                key={index2}
              >
                {rowMeta}
              </td>
            ))}
            <td className="vm-table-cell vm-table-cell_right">
              {row.value}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default TableView;
