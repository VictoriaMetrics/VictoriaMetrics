import React, { FC } from "preact/compat";
import { Data } from "../types";
import LineProgress from "../../../../components/Main/LineProgress/LineProgress";
import { PlayCircleOutlineIcon } from "../../../../components/Main/Icons";
import Button from "../../../../components/Main/Button/Button";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";
import classNames from "classnames";

interface CardinalityTableCells {
  row: Data,
  totalSeries: number;
  totalSeriesPrev: number;
  onActionClick: (name: string) => void;
}

const TableCells: FC<CardinalityTableCells> = ({
  row,
  totalSeries,
  totalSeriesPrev,
  onActionClick
}) => {
  const progress = totalSeries > 0 ? row.value / totalSeries * 100 : -1;
  const progressPrev = totalSeriesPrev > 0 ? row.valuePrev / totalSeriesPrev * 100 : -1;
  const hasProgresses = [progress, progressPrev].some(p => p === -1);

  const formattedValue = row.value.toLocaleString("en-US");
  const formattedPrevValue = row.valuePrev.toLocaleString("en-US");
  const formattedDiff = Math.abs(row.diff).toLocaleString("en-US");

  const diffPercentByTotal = progress - progressPrev;
  const relationPrevDay = hasProgresses ? "" : `${Math.abs(diffPercentByTotal).toFixed(2)}%`;

  const diffPercent = `${Math.abs(row.diffPercent).toFixed(2)}%`;

  const handleActionClick = () => {
    onActionClick(row.name);
  };

  return <>
    <td className="vm-table-cell">
      <span
        className={"vm-link vm-link_colored"}
        onClick={handleActionClick}
      >
        {row.name}
      </span>
    </td>
    <td className="vm-table-cell vm-table-cell_compact">
      {formattedValue}
    </td>
    <td className="vm-table-cell vm-table-cell_compact">
      <Tooltip title={`in relation to the previous day: ${formattedPrevValue}`}>
        <span
          className={classNames({
            "vm-dynamic-number": true,
            "vm-dynamic-number_positive vm-dynamic-number_down": row.diff < 0,
            "vm-dynamic-number_negative vm-dynamic-number_up": row.diff > 0,
          })}
        >
          {formattedDiff}
        </span>
      </Tooltip>
    </td>
    <td className="vm-table-cell vm-table-cell_compact">
      <Tooltip title={`in relation to the previous day: ${formattedPrevValue}`}>
        <div
          className={classNames({
            "vm-dynamic-number": true,
            "vm-dynamic-number_positive vm-dynamic-number_down": row.diff < 0,
            "vm-dynamic-number_negative vm-dynamic-number_up": row.diff > 0,
          })}
        >
          {diffPercent}
        </div>
      </Tooltip>
    </td>
    {progress > 0 && (
      <td className="vm-table-cell">
        <div className="vm-cardinality-panel-table__progress">
          <LineProgress
            value={progress}
            hideValue
          />
          <span className="vm-dynamic-number vm-dynamic-number_static">
            {progress.toFixed(2)}%
          </span>
          <Tooltip title={"in relation to the previous day"}>
            <span
              className={classNames({
                "vm-dynamic-number": true,
                "vm-dynamic-number_no-change": diffPercentByTotal === 0,
                "vm-dynamic-number_positive vm-dynamic-number_down": diffPercentByTotal < 0,
                "vm-dynamic-number_negative vm-dynamic-number_up": diffPercentByTotal > 0,
              })}
            >
              {relationPrevDay}
            </span>
          </Tooltip>
        </div>
      </td>
    )}
    <td
      className="vm-table-cell vm-table-cell_right"
    >
      <div className="vm-table-cell__content">
        <Tooltip title={<span>Filter by <code>`{row.name}`</code></span>}>
          <Button
            variant="text"
            size="small"
            onClick={handleActionClick}
            startIcon={<PlayCircleOutlineIcon/>}
          />
        </Tooltip>
      </div>
    </td>
  </>;
};

export default TableCells;
