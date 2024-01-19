import React, { FC, RefObject } from "react";
import GraphTab from "./GraphTab";
import JsonView from "../../../components/Views/JsonView/JsonView";
import TableTab from "./TableTab";
import { InstantMetricResult, MetricResult } from "../../../api/types";
import { DisplayType } from "../../../types";

type Props = {
  graphData?: MetricResult[];
  liveData?: InstantMetricResult[];
  isHistogram: boolean;
  displayType: DisplayType;
  controlsRef: RefObject<HTMLDivElement>;
}

const CustomPanelTabs: FC<Props> = ({
  graphData,
  liveData,
  isHistogram,
  displayType,
  controlsRef
}) => {
  if (displayType === DisplayType.code && liveData) {
    return <JsonView data={liveData} />;
  }

  if (displayType === DisplayType.table && liveData) {
    return <TableTab
      liveData={liveData}
      controlsRef={controlsRef}
    />;
  }

  if (displayType === DisplayType.chart && graphData) {
    return <GraphTab
      graphData={graphData}
      isHistogram={isHistogram}
      controlsRef={controlsRef}
    />;
  }

  return null;
};

export default CustomPanelTabs;
