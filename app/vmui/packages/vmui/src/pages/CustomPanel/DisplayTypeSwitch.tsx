import React, { FC } from "preact/compat";
import TableChartIcon from "@mui/icons-material/TableChart";
import ShowChartIcon from "@mui/icons-material/ShowChart";
import CodeIcon from "@mui/icons-material/Code";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import { SyntheticEvent } from "react";
import { useCustomPanelDispatch, useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";

export type DisplayType = "table" | "chart" | "code";

type DisplayTab = {
  value: DisplayType
  icon: JSX.Element
  label: string
  prometheusCode: number
}

export const displayTypeTabs: DisplayTab[] = [
  { value: "chart", icon: <ShowChartIcon/>, label: "Graph", prometheusCode: 0 },
  { value: "code", icon: <CodeIcon/>, label: "JSON", prometheusCode: 3 },
  { value: "table", icon: <TableChartIcon/>, label: "Table", prometheusCode: 1 }
];

export const DisplayTypeSwitch: FC = () => {

  const { displayType } = useCustomPanelState();
  const dispatch = useCustomPanelDispatch();

  const handleChange = (event: SyntheticEvent, newValue: DisplayType) => {
    dispatch({ type: "SET_DISPLAY_TYPE", payload: newValue ?? displayType });
  };

  return <Tabs
    value={displayType}
    onChange={handleChange}
    sx={{ minHeight: "0", marginBottom: "-1px" }}
  >
    {displayTypeTabs.map(t =>
      <Tab
        key={t.value}
        icon={t.icon}
        iconPosition="start"
        label={t.label}
        value={t.value}
        sx={{ minHeight: "41px" }}
      />)}
  </Tabs>;
};
