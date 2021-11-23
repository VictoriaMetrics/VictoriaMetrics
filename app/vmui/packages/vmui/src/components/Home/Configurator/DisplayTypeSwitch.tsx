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
    padding: 6,
    color: "white",
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
      <ShowChartIcon/>&nbsp;Query Range as Chart
    </StylizedToggleButton>
    <StylizedToggleButton value="code" aria-label="display as code">
      <CodeIcon/>&nbsp;Instant Query as JSON
    </StylizedToggleButton>
    <StylizedToggleButton value="table" aria-label="display as table">
      <TableChartIcon/>&nbsp;Instant Query as Table
    </StylizedToggleButton>
  </ToggleButtonGroup>;
};