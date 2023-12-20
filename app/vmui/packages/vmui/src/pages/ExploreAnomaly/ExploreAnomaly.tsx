import React, { FC, useMemo, useRef } from "preact/compat";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import useEventListener from "../../hooks/useEventListener";
import "../CustomPanel/style.scss";
import ExploreAnomalyHeader from "./ExploreAnomalyHeader/ExploreAnomalyHeader";
import Alert from "../../components/Main/Alert/Alert";
import { extractFields } from "../../utils/uplot";
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
    const group = queries.length + 1;
    const realData = graphData.filter(d => d.group === 1);
    const upperData = graphData.filter(d => d.group === 3);
    const lowerData = graphData.filter(d => d.group === 4);
    const anomalyData: MetricResult[] = realData.map((d) => ({
      group,
      metric: { ...d.metric, __name__: ForecastType.anomaly },
      values: d.values.filter(([t, v]) => {
        const id = extractFields(d.metric);
        const upperDataByLabels = upperData.find(du => extractFields(du.metric) === id);
        const lowerDataByLabels = lowerData.find(du => extractFields(du.metric) === id);
        if (!upperDataByLabels || !lowerDataByLabels) return false;
        const max = upperDataByLabels.values.find(([tMax]) => tMax === t) as [number, string];
        const min = lowerDataByLabels.values.find(([tMin]) => tMin === t) as [number, string];
        const num = v && promValueToNumber(v);
        const numMin = min && promValueToNumber(min[1]);
        const numMax = max && promValueToNumber(max[1]);
        return num < numMin || num > numMax;
      })
    }));
    return graphData.concat(anomalyData);
  }, [graphData]);

  const onChangeFilter = (expr: Record<string, string>) => {
    const { __name__ = "", ...labelValue } = expr;
    let prefix = __name__.replace(/y|_y/, "");
    if (prefix) prefix += "_";
    const metrics = [__name__, ForecastType.yhat, ForecastType.yhatUpper, ForecastType.yhatLower];
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
