import { FC, createPortal, useCallback, RefObject } from "preact/compat";
import GraphView from "../../../components/Views/GraphView/GraphView";
import GraphTips from "../../../components/Chart/GraphTips/GraphTips";
import GraphSettings from "../../../components/Configurators/GraphSettings/GraphSettings";
import { AxisRange } from "../../../state/graph/reducer";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { MetricResult } from "../../../api/types";

type Props = {
  isHistogram: boolean;
  graphData: MetricResult[];
  controlsRef: RefObject<HTMLDivElement>;
  isAnomalyView?: boolean;
}

const GraphTab: FC<Props> = ({ isHistogram, graphData, controlsRef, isAnomalyView }) => {
  const { isMobile } = useDeviceDetect();

  const { customStep, yaxis, spanGaps, showAllPoints } = useGraphState();
  const { period } = useTimeState();
  const { query } = useQueryState();

  const timeDispatch = useTimeDispatch();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = useCallback((limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  }, [graphDispatch]);

  const toggleEnableLimits = useCallback(() => {
    graphDispatch({ type: "TOGGLE_ENABLE_YAXIS_LIMITS" });
  }, [graphDispatch]);

  const setSpanGaps = useCallback((value: boolean) => {
    graphDispatch({ type: "SET_SPAN_GAPS", payload: value });
  }, [graphDispatch]);

  const setPeriod = useCallback(({ from, to }: { from: Date; to: Date }) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  }, [timeDispatch]);

  const setShowPoints = useCallback((value: boolean) => {
    graphDispatch({ type: "SET_SHOW_POINTS", payload: value });
  }, [graphDispatch]);

  const controls = (
    <div className="vm-custom-panel-body-header__graph-controls">
      <GraphTips/>
      <GraphSettings
        data={graphData}
        yaxis={yaxis}
        isHistogram={isHistogram}
        setYaxisLimits={setYaxisLimits}
        toggleEnableLimits={toggleEnableLimits}
        spanGaps={{ value: spanGaps, onChange: setSpanGaps }}
        showAllPoints={{ value: showAllPoints, onChange: setShowPoints }}
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
        isAnomalyView={isAnomalyView}
        spanGaps={spanGaps}
        showAllPoints={showAllPoints}
      />
    </>
  );
};

export default GraphTab;
