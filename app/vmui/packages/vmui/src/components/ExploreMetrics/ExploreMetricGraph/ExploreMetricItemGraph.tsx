import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { useFetchQuery } from "../../../hooks/useFetchQuery";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import GraphView from "../../Views/GraphView/GraphView";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { AxisRange } from "../../../state/graph/reducer";
import Spinner from "../../Main/Spinner/Spinner";
import Alert from "../../Main/Alert/Alert";
import Button from "../../Main/Button/Button";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { getDurationFromMilliseconds, getSecondsFromDuration, getStepFromDuration } from "../../../utils/time";
import useBoolean from "../../../hooks/useBoolean";

interface ExploreMetricItemGraphProps {
  name: string,
  job: string,
  instance: string,
  rateEnabled: boolean,
  isBucket: boolean,
  height?: number
}

const ExploreMetricItem: FC<ExploreMetricItemGraphProps> = ({
  name,
  job,
  instance,
  rateEnabled,
  isBucket,
  height,
}) => {
  const { isMobile } = useDeviceDetect();
  const { customStep, yaxis } = useGraphState();
  const { period } = useTimeState();
  const graphDispatch = useGraphDispatch();
  const timeDispatch = useTimeDispatch();

  const defaultStep = getStepFromDuration(period.end - period.start);
  const stepSeconds = getSecondsFromDuration(customStep);
  const heatmapStep = getDurationFromMilliseconds(stepSeconds * 10 * 1000);
  const [isHeatmap, setIsHeatmap] = useState(false);
  const step = isHeatmap && customStep === defaultStep ? heatmapStep : customStep;

  const {
    value: showAllSeries,
    setTrue: handleShowAll,
  } = useBoolean(false);

  const query = useMemo(() => {
    const params = Object.entries({ job, instance })
      .filter(val => val[1])
      .map(([key, val]) => `${key}=${JSON.stringify(val)}`);
    params.push(`__name__=${JSON.stringify(name)}`);
    if (name == "node_cpu_seconds_total") {
      // This is hack for filtering out free cpu for widely used metric :)
      params.push("mode!=\"idle\"");
    }

    const base = `{${params.join(",")}}`;
    if (isBucket) {
      return `sum(rate(${base})) by (vmrange, le)`;
    }
    const queryBase = rateEnabled ? `rollup_rate(${base})` : `rollup(${base})`;
    return `
with (q = ${queryBase}) (
  alias(min(label_match(q, "rollup", "min")), "min"),
  alias(max(label_match(q, "rollup", "max")), "max"),
  alias(avg(label_match(q, "rollup", "avg")), "avg"),
)`;
  }, [name, job, instance, rateEnabled, isBucket]);

  const { isLoading, graphData, error, queryErrors, warning, isHistogram } = useFetchQuery({
    predefinedQuery: [query],
    visible: true,
    customStep: step,
    showAllSeries
  });

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  useEffect(() => {
    setIsHeatmap(isHistogram);
  }, [isHistogram]);

  return (
    <div
      className={classNames({
        "vm-explore-metrics-graph": true,
        "vm-explore-metrics-graph_mobile": isMobile
      })}
    >
      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {queryErrors[0] && <Alert variant="error">{queryErrors[0]}</Alert>}
      {warning && <Alert variant="warning">
        <div className="vm-explore-metrics-graph__warning">
          <p>{warning}</p>
          <Button
            color="warning"
            variant="outlined"
            onClick={handleShowAll}
          >
              Show all
          </Button>
        </div>
      </Alert>}
      {graphData && period && (
        <GraphView
          data={graphData}
          period={period}
          customStep={step}
          query={[query]}
          yaxis={yaxis}
          setYaxisLimits={setYaxisLimits}
          setPeriod={setPeriod}
          showLegend={false}
          height={height}
          isHistogram={isHistogram}
        />
      )}
    </div>
  );
};

export default ExploreMetricItem;
