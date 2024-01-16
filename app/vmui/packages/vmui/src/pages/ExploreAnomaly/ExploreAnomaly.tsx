import React, { FC, useMemo, useRef } from "preact/compat";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import useEventListener from "../../hooks/useEventListener";
import "../CustomPanel/style.scss";
import ExploreAnomalyHeader from "./ExploreAnomalyHeader/ExploreAnomalyHeader";
import Alert from "../../components/Main/Alert/Alert";
import { extractFields, isForecast } from "../../utils/uplot";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import Spinner from "../../components/Main/Spinner/Spinner";
import GraphTab from "../CustomPanel/CustomPanelTabs/GraphTab";
import { useGraphState } from "../../state/graph/GraphStateContext";
import { MetricResult } from "../../api/types";
import { promValueToNumber } from "../../utils/metric";
import { ForecastType } from "../../types";
import { useFetchAnomalySeries } from "./hooks/useFetchAnomalySeries";
import { useQueryDispatch } from "../../state/query/QueryStateContext";
import { useTimeDispatch } from "../../state/time/TimeStateContext";

const anomalySeries = [
  ForecastType.yhat,
  ForecastType.yhatUpper,
  ForecastType.yhatLower,
  ForecastType.anomalyScore
];

// Hardcoded to 1.0 for now; consider adding a UI slider for threshold adjustment in the future.
const ANOMALY_SCORE_THRESHOLD = 1;

const ExploreAnomaly: FC = () => {
  const { isMobile } = useDeviceDetect();

  const queryDispatch = useQueryDispatch();
  const timeDispatch = useTimeDispatch();
  const { series, error: errorSeries, isLoading: isAnomalySeriesLoading } = useFetchAnomalySeries();
  const queries = useMemo(() => series ? Object.keys(series) : [], [series]);

  const controlsRef = useRef<HTMLDivElement>(null);
  const { customStep } = useGraphState();

  const { graphData, error, queryErrors, isHistogram, isLoading: isGraphDataLoading } = useFetchQuery({
    visible: true,
    customStep,
    showAllSeries: true,
  });

  const data = useMemo(() => {
    if (!graphData) return;
    const detectedData = graphData.map(d => ({ ...isForecast(d.metric), ...d }));
    const realData = detectedData.filter(d => d.value === null);
    const anomalyScoreData = detectedData.filter(d => d.isAnomalyScore);
    const anomalyData: MetricResult[] = realData.map((d) => {
      const id = extractFields(d.metric);
      const anomalyScoreDataByLabels = anomalyScoreData.find(du => extractFields(du.metric) === id);

      return {
        group: queries.length + 1,
        metric: { ...d.metric, __name__: ForecastType.anomaly },
        values: d.values.filter(([t]) => {
          if (!anomalyScoreDataByLabels) return false;
          const anomalyScore = anomalyScoreDataByLabels.values.find(([tMax]) => tMax === t) as [number, string];
          return anomalyScore && promValueToNumber(anomalyScore[1]) > ANOMALY_SCORE_THRESHOLD;
        })
      };
    });
    return graphData.filter(d => d.group !== anomalyScoreData[0]?.group).concat(anomalyData);
  }, [graphData]);

  const onChangeFilter = (expr: Record<string, string>) => {
    const { __name__ = "", ...labelValue } = expr;
    let prefix = __name__.replace(/y|_y/, "");
    if (prefix) prefix += "_";
    const metrics = [__name__, ...anomalySeries];
    const filters = Object.entries(labelValue).map(([key, value]) => `${key}="${value}"`).join(",");
    const queries = metrics.map((m, i) => `${i ? prefix : ""}${m}{${filters}}`);
    queryDispatch({ type: "SET_QUERY", payload: queries });
    timeDispatch({ type: "RUN_QUERY" });
  };

  const handleChangePopstate = () => window.location.reload();
  useEventListener("popstate", handleChangePopstate);

  return (
    <div
      className={classNames({
        "vm-custom-panel": true,
        "vm-custom-panel_mobile": isMobile,
      })}
    >
      <ExploreAnomalyHeader
        queries={queries}
        series={series}
        onChange={onChangeFilter}
      />
      {(isGraphDataLoading || isAnomalySeriesLoading) && <Spinner />}
      {(error || errorSeries) && <Alert variant="error">{error || errorSeries}</Alert>}
      {!error && !errorSeries && queryErrors?.[0] && <Alert variant="error">{queryErrors[0]}</Alert>}
      <div
        className={classNames({
          "vm-custom-panel-body": true,
          "vm-custom-panel-body_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div
          className="vm-custom-panel-body-header"
          ref={controlsRef}
        >
          <div/>
        </div>
        {data && (
          <GraphTab
            graphData={data}
            isHistogram={isHistogram}
            controlsRef={controlsRef}
            anomalyView={true}
          />
        )}
      </div>
    </div>
  );
};

export default ExploreAnomaly;
