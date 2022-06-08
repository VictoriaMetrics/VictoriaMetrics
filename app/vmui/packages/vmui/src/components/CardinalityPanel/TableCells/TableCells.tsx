import {TableCell, ButtonGroup} from "@mui/material";
import {Data} from "../../Table/types";
import {BorderLinearProgressWithLabel} from "../../BorderLineProgress/BorderLinearProgress";
import React from "preact/compat";
import IconButton from "@mui/material/IconButton";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import Tooltip from "@mui/material/Tooltip";
import {SyntheticEvent} from "react";
import dayjs from "dayjs";

export const tableCells = (
  row: Data,
  date: string | null,
  onFilterClick: (e: SyntheticEvent) => void) => {
  const pathname = window.location.pathname;
  const withday = dayjs(date).add(1, "day").toDate();
  return Object.keys(row).map((key, idx) => {
    if (idx === 0) {
      return (<TableCell component="th" scope="row" key={key}>
        {row[key as keyof Data]}
      </TableCell>);
    }
    if (key === "progressValue") {
      return (
        <TableCell key={key}>
          <BorderLinearProgressWithLabel
            variant="determinate"
            value={row[key as keyof Data] as number}
          />
        </TableCell>
      );
    }
    if (key === "actions") {
      const title = `Filter by ${row.name}`;
      return (<TableCell key={key}>
        <ButtonGroup variant="contained">
          <Tooltip title={title}>
            <IconButton
              id={row.name}
              onClick={onFilterClick}
              sx={{height: "20px", width: "20px"}}>
              <PlayCircleOutlineIcon/>
            </IconButton>
          </Tooltip>
        </ButtonGroup>
      </TableCell>);
    }
    return (<TableCell key={key}>{row[key as keyof Data]}</TableCell>);
  });
};
