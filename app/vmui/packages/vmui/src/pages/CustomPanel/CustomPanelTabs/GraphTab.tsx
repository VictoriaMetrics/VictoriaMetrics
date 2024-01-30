import React, { FC } from "react";
import GraphView from "../../../components/Views/GraphView/GraphView";
import GraphTips from "../../../components/Chart/GraphTips/GraphTips";
import GraphSettings from "../../../components/Configurators/GraphSettings/GraphSettings";
import { AxisRange } from "../../../state/graph/reducer";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { MetricResult } from "../../../api/types";
import { createPortal } from "preact/compat";

type Props = {
  isHistogram: boolean;
  graphData: MetricResult[];
  controlsRef: React.RefObject<HTMLDivElement>;
  anomalyView?: boolean;
}

const GraphTab: FC<Props> = ({ isHistogram, graphData, controlsRef, anomalyView }) => {
  const { isMobile } = useDeviceDetect();

  const { customStep, yaxis } = useGraphState();
  const { period } = useTimeState();
  const { query } = useQueryState();

  const timeDispatch = useTimeDispatch();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const toggleEnableLimits = () => {
    graphDispatch({ type: "TOGGLE_ENABLE_YAXIS_LIMITS" });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  const controls = (
    <div className="vm-custom-panel-body-header__graph-controls">
      <GraphTips/>
      <GraphSettings
        yaxis={yaxis}
        setYaxisLimits={setYaxisLimits}
        toggleEnableLimits={toggleEnableLimits}
      />
    </div>
  );

  return (
    <>
      {controlsRef.current && createPortal(controls, controlsRef.current)}
      <GraphView
        data={graphData}
        period={period}
        customStep={customStep}
        query={query}
        yaxis={yaxis}
        setYaxisLimits={setYaxisLimits}
        setPeriod={setPeriod}
        height={isMobile ? window.innerHeight * 0.5 : 500}
        isHistogram={isHistogram}
        anomalyView={anomalyView}
      />
    </>
  );
};

export default GraphTab;
