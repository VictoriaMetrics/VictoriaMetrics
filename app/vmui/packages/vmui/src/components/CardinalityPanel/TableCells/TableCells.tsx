import {SyntheticEvent} from "react";
import React, {FC} from "preact/compat";
import {TableCell, ButtonGroup} from "@mui/material";
import {Data} from "../../Table/types";
import {BorderLinearProgressWithLabel} from "../../BorderLineProgress/BorderLinearProgress";
import IconButton from "@mui/material/IconButton";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import Tooltip from "@mui/material/Tooltip";

interface CardinalityTableCells {
  row: Data,
  totalSeries: number;
  onActionClick: (e: SyntheticEvent) => void;
}

const TableCells: FC<CardinalityTableCells> = ({ row, totalSeries, onActionClick }) => {
  const progress = totalSeries > 0 ? row.value / totalSeries * 100 : -1;
  return <>
    <TableCell key={row.name}>{row.name}</TableCell>
    <TableCell key={row.value}>{row.value}</TableCell>
    {progress > 0 ? <TableCell key={row.progressValue}>
      <BorderLinearProgressWithLabel variant="determinate" value={progress} />
    </TableCell> : null}
    <TableCell key={"action"}>
      <ButtonGroup variant="contained">
        <Tooltip title={`Filter by ${row.name}`}>
          <IconButton
            id={row.name}
            onClick={onActionClick}
            sx={{height: "20px", width: "20px"}}>
            <PlayCircleOutlineIcon/>
          </IconButton>
        </Tooltip>
      </ButtonGroup>
    </TableCell>
  </>;
};

export default TableCells;
