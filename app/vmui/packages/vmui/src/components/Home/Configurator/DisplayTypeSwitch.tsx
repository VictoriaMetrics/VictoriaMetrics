import React, {FC} from "react";

import TableChartIcon from "@mui/icons-material/TableChart";
import ShowChartIcon from "@mui/icons-material/ShowChart";
import CodeIcon from "@mui/icons-material/Code";

import { ToggleButton, ToggleButtonGroup } from "@mui/material";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import withStyles from "@mui/styles/withStyles";

export type DisplayType = "table" | "chart" | "code";

const StylizedToggleButton = withStyles({
  root: {
    display: "grid",
    gridTemplateColumns: "18px auto",
    gridGap: 6,
    padding: "8px 12px",
    color: "white",
    lineHeight: "19px",
    "&.Mui-selected": {
      color: "white"
    }
  }
})(ToggleButton);

export const DisplayTypeSwitch: FC = () => {

  const {displayType} = useAppState();
  const dispatch = useAppDispatch();

  return <ToggleButtonGroup
    value={displayType}
    exclusive
    onChange={
      (e, val) =>
      // Toggle Button Group returns null in case of click on selected element, avoiding it
        dispatch({type: "SET_DISPLAY_TYPE", payload: val ?? displayType})
    }>
    <StylizedToggleButton value="chart" aria-label="display as chart">
      <ShowChartIcon/><span>Query Range as Chart</span>
    </StylizedToggleButton>
    <StylizedToggleButton value="code" aria-label="display as code">
      <CodeIcon/><span>Instant Query as JSON</span>
    </StylizedToggleButton>
    <StylizedToggleButton value="table" aria-label="display as table">
      <TableChartIcon/><span>Instant Query as Table</span>
    </StylizedToggleButton>
  </ToggleButtonGroup>;
};