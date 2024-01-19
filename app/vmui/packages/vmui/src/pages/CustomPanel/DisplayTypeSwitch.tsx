import React, { FC } from "preact/compat";
import { useCustomPanelDispatch, useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { ChartIcon, CodeIcon, TableIcon } from "../../components/Main/Icons";
import Tabs from "../../components/Main/Tabs/Tabs";
import { DisplayType } from "../../types";

type DisplayTab = {
  value: DisplayType
  icon: JSX.Element
  label: string
  prometheusCode: number
}

export const displayTypeTabs: DisplayTab[] = [
  { value: DisplayType.chart, icon: <ChartIcon/>, label: "Graph", prometheusCode: 0 },
  { value: DisplayType.code, icon: <CodeIcon/>, label: "JSON", prometheusCode: 3 },
  { value: DisplayType.table, icon: <TableIcon/>, label: "Table", prometheusCode: 1 }
];

export const DisplayTypeSwitch: FC = () => {

  const { displayType } = useCustomPanelState();
  const dispatch = useCustomPanelDispatch();

  const handleChange = (newValue: string) => {
    dispatch({ type: "SET_DISPLAY_TYPE", payload: newValue as DisplayType ?? displayType });
  };

  return (
    <Tabs
      activeItem={displayType}
      items={displayTypeTabs}
      onChange={handleChange}
    />
  );
};
