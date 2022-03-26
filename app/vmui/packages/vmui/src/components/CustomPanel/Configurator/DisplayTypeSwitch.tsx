import React, {FC} from "preact/compat";
import TableChartIcon from "@mui/icons-material/TableChart";
import ShowChartIcon from "@mui/icons-material/ShowChart";
import CodeIcon from "@mui/icons-material/Code";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import {SyntheticEvent} from "react";

export type DisplayType = "table" | "chart" | "code";

const tabs = [
  {value: "chart", icon: <ShowChartIcon/>, label: "Graph"},
  {value: "code", icon: <CodeIcon/>, label: "JSON"},
  {value: "table", icon: <TableChartIcon/>, label: "Table"}
];

export const DisplayTypeSwitch: FC = () => {

  const {displayType} = useAppState();
  const dispatch = useAppDispatch();

  const handleChange = (event: SyntheticEvent, newValue: DisplayType) => {
    dispatch({type: "SET_DISPLAY_TYPE", payload: newValue ?? displayType});
  };

  return <Tabs
    value={displayType}
    onChange={handleChange}
    sx={{minHeight: "0", marginBottom: "-1px"}}
  >
    {tabs.map(t =>
      <Tab key={t.value}
        icon={t.icon}
        iconPosition="start"
        label={t.label} value={t.value}
        sx={{minHeight: "41px"}}
      />)}
  </Tabs>;
};