import { SyntheticEvent } from "react";
import React, { FC } from "preact/compat";
import { Data } from "../types";
import LineProgress from "../../../../components/Main/LineProgress/LineProgress";
import { PlayCircleOutlineIcon } from "../../../../components/Main/Icons";
import Button from "../../../../components/Main/Button/Button";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";

interface CardinalityTableCells {
  row: Data,
  totalSeries: number;
  onActionClick: (e: SyntheticEvent) => void;
}

const TableCells: FC<CardinalityTableCells> = ({ row, totalSeries, onActionClick }) => {
  const progress = totalSeries > 0 ? row.value / totalSeries * 100 : -1;

  return <>
    <td
      className="vm-table-cell"
      key={row.name}
    >
      {row.name}
    </td>
    <td
      className="vm-table-cell"
      key={row.value}
    >
      {row.value}
    </td>
    {progress > 0 && (
      <td
        className="vm-table-cell"
        key={row.progressValue}
      >
        <LineProgress value={progress}/>
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
            onClick={onActionClick}
          >
            <PlayCircleOutlineIcon/>
          </Button>
        </Tooltip>
      </div>
    </td>
  </>;
};

export default TableCells;
