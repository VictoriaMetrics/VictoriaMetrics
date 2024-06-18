import React, { FC, useMemo, useRef, useState } from "preact/compat";
import { InstantMetricResult } from "../../../api/types";
import { InstantDataSeries } from "../../../types";
import { useSortedCategories } from "../../../hooks/useSortedCategories";
import Alert from "../../Main/Alert/Alert";
import classNames from "classnames";
import { ArrowDropDownIcon, CopyIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { getNameForMetric } from "../../../utils/metric";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

export interface GraphViewProps {
  data: InstantMetricResult[];
  displayColumns?: string[]
}

const TableView: FC<GraphViewProps> = ({ data, displayColumns }) => {
  const copyToClipboard = useCopyToClipboard();
  const { isMobile } = useDeviceDetect();

  const { tableCompact } = useCustomPanelState();
  const tableRef = useRef<HTMLTableElement>(null);

  const [orderBy, setOrderBy] = useState("");
  const [orderDir, setOrderDir] = useState<"asc" | "desc">("asc");

  const sortedColumns = (tableCompact
    ? useSortedCategories([{ group: 0, metric: { "Data": "Data" } }], ["Data"])
    : useSortedCategories(data, displayColumns)
  );

  const getCopyValue = (metric: { [p: string]: string }) => {
    const { __name__, ...fields } = metric;
    if (!__name__ && !Object.keys(fields).length) return "";
    if (!__name__) return `${JSON.stringify(fields)}`;
    return `${__name__} ${JSON.stringify(fields)}`;
  };

  const groups = new Set(data?.map(d => d.group));
  const showQueryName = groups.size > 1;

  const rows: InstantDataSeries[] = useMemo(() => {
    const rows = data?.map(d => ({
      metadata: sortedColumns.map(c => (tableCompact
        ? getNameForMetric(d, "", showQueryName)
        : (d.metric[c.key] || "-")
      )),
      value: d.value ? d.value[1] : "-",
      values: d.values ? d.values.map(([time, val]) => `${val} @${time}`) : [],
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

  const createSortHandler = (key: string) => () => {
    sortHandler(key);
  };

  const createCopyHandler = (copyValue: string) => async () => {
    await copyToClipboard(copyValue, "Row has been copied");
  };

  if (!rows.length) return <Alert variant="warning">No data to show</Alert>;

  return (
    <div
      className={classNames({
        "vm-table-view": true,
        "vm-table-view_mobile": isMobile,
      })}
    >
      <table
        className="vm-table"
        ref={tableRef}
      >
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
              <td className="vm-table-cell vm-table-cell_right vm-table-cell_no-wrap">
                {!row.values.length ? row.value : row.values.map(val => <p key={val}>{val}</p>)}
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
    </div>
  );
};

export default TableView;
