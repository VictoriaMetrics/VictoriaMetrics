import { SyntheticEvent } from "react";
import React, { FC } from "preact/compat";
import { Data } from "../../../components/Main/Table/types";
import { BorderLinearProgressWithLabel } from "../../../components/Main/BorderLineProgress/BorderLinearProgress";
import { PlayCircleOutlineIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";

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
      <BorderLinearProgressWithLabel
        value={progress}
      />
    </td> : null}
    <td key={"action"}>
      <div>
        {/*<Tooltip title={`Filter by ${row.name}`}>*/}
        <Button
          onClick={onActionClick}
        >
          <PlayCircleOutlineIcon/>
        </Button>
        {/*</Tooltip>*/}
      </div>
    </td>
  </>;
};

export default TableCells;
