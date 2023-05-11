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

  const diffPercent = progress - progressPrev;
  const relationPrevDay = hasProgresses ? "" : `${diffPercent.toFixed(2)}%`;

  const handleActionClick = () => {
    onActionClick(row.name);
  };

  return <>
    <td
      className="vm-table-cell"
      key={row.name}
    >
      <span
        className={"vm-link vm-link_colored"}
        onClick={handleActionClick}
      >
        {row.name}
      </span>
    </td>
    <td
      className="vm-table-cell"
      key={row.value}
    >
      {row.value}

      {!!row.diff && (
        <Tooltip title={`in relation to the previous day: ${row.valuePrev}`}>
          <span
            className={classNames({
              "vm-dynamic-number": true,
              "vm-dynamic-number_positive": row.diff < 0,
              "vm-dynamic-number_negative": row.diff > 0,
            })}
          >
          &nbsp;{row.diff > 0 ? "+" : ""}{row.diff}
          </span>
        </Tooltip>
      )}
    </td>
    {progress > 0 && (
      <td
        className="vm-table-cell"
        key={row.progressValue}
      >
        <div className="vm-cardinality-panel-table__progress">
          <LineProgress value={progress}/>
          {relationPrevDay && (
            <Tooltip title={"in relation to the previous day"}>
              <span
                className={classNames({
                  "vm-dynamic-number": true,
                  "vm-dynamic-number_positive vm-dynamic-number_down": diffPercent < 0,
                  "vm-dynamic-number_negative vm-dynamic-number_up": diffPercent > 0,
                })}
              >
                {relationPrevDay}
              </span>
            </Tooltip>
          )}
        </div>
      </td>
    )}
    <td
      className="vm-table-cell vm-table-cell_right"
      key={"action"}
    >
      <div className="vm-table-cell__content">
        <Tooltip title={`Filter by ${row.name}`}>
          <Button
            variant="text"
            size="small"
            onClick={handleActionClick}
          >
            <PlayCircleOutlineIcon/>
          </Button>
        </Tooltip>
      </div>
    </td>
  </>;
};

export default TableCells;
