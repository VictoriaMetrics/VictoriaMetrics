import React, { FC, useMemo } from "preact/compat";
import { LegendProps } from "../LegendGroup";
import "./style.scss";
import { LegendItemType } from "../../../../../types";
import { MouseEvent } from "react";
import classNames from "classnames";
import get from "lodash.get";
import { STATS_ORDER } from "../../../../../constants/graph";
import { useShowStats } from "../hooks/useShowStats";

const statsColumns = STATS_ORDER.map(k => ({
  key: `statsFormatted.${k}`,
  title: k
}));

const LegendTable: FC<LegendProps> = ({ labels, duplicateFields, onChange }) => {
  const { hideStats } = useShowStats();

  const stats = hideStats ? [] : statsColumns;

  const fields = useMemo(() => {
    const fields = [...new Set(labels.flatMap(item => Object.keys(item.freeFormFields)))]
      .map(f => ({ key: `freeFormFields.${f}`, title: f }));

    return duplicateFields?.length
      ? fields.filter(f => !duplicateFields.includes(f.title))
      : fields;
  }, [labels, duplicateFields]);

  const columns = fields.concat(stats);

  const createHandlerClick = (legend: LegendItemType) => (e: MouseEvent<HTMLTableRowElement>) => {
    onChange && onChange(legend, e.ctrlKey || e.metaKey);
  };

  return (
    <div className="vm-legend-table__wrapper">
      <table className="vm-legend-table">
        <thead className="vm-legend-table-thead">
          <tr className="vm-legend-table-row vm-legend-table_thead">
            <th className="vm-legend-table-col vm-legend-table-col_marker vm-legend-table-col_thead"/>
            {columns.map((col) => (
              <th
                key={col.key}
                className="vm-legend-table-col vm-legend-table-col_thead"
              >
                {col.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="vm-legend-table-tbody">
          {labels.map(row => (
            <tr
              key={row.label}
              className={classNames({
                "vm-legend-table-row": true,
                "vm-legend-table-row_tbody": true,
                "vm-legend-table-row_exclude": !row.checked
              })}
              onClick={createHandlerClick(row)}
            >
              <td className="vm-legend-table-col vm-legend-table-col_marker">
                <div
                  className="vm-legend-item__marker"
                  style={{ backgroundColor: row.color }}
                />
              </td>
              {columns.map((col) => (
                <td
                  key={`${col.key}_${row.label}`}
                  className="vm-legend-table-col"
                >
                  <span className="vm-legend-table-col__content">
                    {get(row, col.key)}
                  </span>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default LegendTable;
