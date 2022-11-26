import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { InstantMetricResult } from "../../../api/types";
import { InstantDataSeries } from "../../../types";
import { useSortedCategories } from "../../../hooks/useSortedCategories";
import Alert from "../../Main/Alert/Alert";
import classNames from "classnames";
import { ArrowDropDownIcon, CopyIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";
import { useSnack } from "../../../contexts/Snackbar";
import { getNameForMetric } from "../../../utils/metric";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import "./style.scss";
import useResize from "../../../hooks/useResize";

export interface GraphViewProps {
  data: InstantMetricResult[];
  displayColumns?: string[]
}

const TableView: FC<GraphViewProps> = ({ data, displayColumns }) => {
  const { showInfoMessage } = useSnack();

  const { tableCompact } = useCustomPanelState();
  const windowSize = useResize(document.body);
  const tableRef = useRef<HTMLTableElement>(null);
  const [tableTop, setTableTop] = useState(0);
  const [headTop, setHeadTop] = useState(0);

  const [orderBy, setOrderBy] = useState("");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("asc");

  const sortedColumns = (tableCompact
    ? useSortedCategories([{ group: 0, metric: { "Data": "Data" } }], ["Data"])
    : useSortedCategories(data, displayColumns)
  );

  const getCopyValue = (metric: { [p: string]: string }) => {
    const { __name__, ...fields } = metric;
    if (!__name__ && !Object.keys(fields).length) return "";
    return `${__name__} ${JSON.stringify(fields)}`;
  };

  const rows: InstantDataSeries[] = useMemo(() => {
    const rows = data?.map(d => ({
      metadata: sortedColumns.map(c => (tableCompact
        ? getNameForMetric(d, undefined, "=", true)
        : (d.metric[c.key] || "-")
      )),
      value: d.value ? d.value[1] : "-",
      copyValue: getCopyValue(d.metric)
    }));
    const orderByValue = orderBy === "Value";
    const rowIndex = sortedColumns.findIndex(c => c.key === orderBy);
    if (!orderByValue && rowIndex === -1) return rows;
    return rows.sort((a, b) => {
      const n1 = orderByValue ? Number(a.value) : a.metadata[rowIndex];
      const n2 = orderByValue ? Number(b.value) : b.metadata[rowIndex];
      const asc = orderDir === "asc" ? n1 < n2 : n1 > n2;
      return asc ? -1 : 1;
    });
  }, [sortedColumns, data, orderBy, orderDir, tableCompact]);

  const hasCopyValue = useMemo(() => rows.some(r => r.copyValue), [rows]);

  const sortHandler = (key: string) => {
    setOrderDir((prev) => prev === "asc" && orderBy === key ? "desc" : "asc");
    setOrderBy(key);
  };

  const copyHandler = async (copyValue: string) => {
    await navigator.clipboard.writeText(copyValue);
    showInfoMessage({ text: "Row has been copied", type: "success" });
  };

  const createSortHandler = (key: string) => () => {
    sortHandler(key);
  };

  const createCopyHandler = (copyValue: string) => () => {
    copyHandler(copyValue);
  };

  const handleScroll = () => {
    if (!tableRef.current) return;
    const { top } = tableRef.current.getBoundingClientRect();
    setHeadTop(top < 0 ? window.scrollY - tableTop : 0);
  };

  useEffect(() => {
    window.addEventListener("scroll", handleScroll);

    return () => {
      window.removeEventListener("scroll", handleScroll);
    };
  }, [tableRef, tableTop, windowSize]);

  useEffect(() => {
    if (!tableRef.current) return;
    const { top } = tableRef.current.getBoundingClientRect();
    setTableTop(top + window.scrollY);
  }, [tableRef, windowSize]);

  if (!rows.length) return <Alert variant="warning">No data to show</Alert>;

  return (
    <div className="vm-table-view">
      <table
        className="vm-table"
        ref={tableRef}
      >
        <thead className="vm-table-header">
          <tr
            className="vm-table__row vm-table__row_header"
            style={{ transform: `translateY(${headTop}px)` }}
          >
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
            {hasCopyValue && <td className="vm-table-cell vm-table-cell_header"/>}
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
                    "vm-table-cell_gray": rows[index - 1] && rows[index - 1].metadata[index2] === rowMeta
                  })}
                  key={index2}
                >
                  {rowMeta}
                </td>
              ))}
              <td className="vm-table-cell vm-table-cell_right">
                {row.value}
              </td>
              {hasCopyValue && (
                <td className="vm-table-cell vm-table-cell_right">
                  {row.copyValue && (
                    <div className="vm-table-cell__content">
                      <Tooltip title="Copy row">
                        <Button
                          variant="text"
                          color="gray"
                          size="small"
                          startIcon={<CopyIcon/>}
                          onClick={createCopyHandler(row.copyValue)}
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
    </div>
  );
};

export default TableView;
