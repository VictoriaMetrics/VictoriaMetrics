import { SyntheticEvent } from "react";
import React, { FC } from "preact/compat";
import { Data } from "../../../components/Main/Table/types";
import LineProgress from "../../../components/Main/LineProgress/LineProgress";
import { PlayCircleOutlineIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";

interface CardinalityTableCells {
  row: Data,
  totalSeries: number;
  onActionClick: (e: SyntheticEvent) => void;
}

const TableCells: FC<CardinalityTableCells> = ({ row, totalSeries, onActionClick }) => {
  const progress = totalSeries > 0 ? row.value / totalSeries * 100 : -1;
  return <>
    <td key={row.name}>{row.name}</td>
    <td key={row.value}>{row.value}</td>
    {progress > 0 ? <td key={row.progressValue}>
      <LineProgress value={progress}/>
    </td> : null}
    <td key={"action"}>
      <div>
        <Tooltip title={`Filter by ${row.name}`}>
          <Button
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
