import React, {FC} from "react";

import TableChartIcon from "@material-ui/icons/TableChart";
import ShowChartIcon from "@material-ui/icons/ShowChart";
import CodeIcon from "@material-ui/icons/Code";

import {ToggleButton, ToggleButtonGroup} from "@material-ui/lab";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import {withStyles} from "@material-ui/core";

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