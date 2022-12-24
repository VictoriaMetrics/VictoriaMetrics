import React, { FC, useMemo, useState } from "preact/compat";
import { useFetchQuery } from "../../../hooks/useFetchQuery";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import GraphView from "../../../components/Views/GraphView/GraphView";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { AxisRange } from "../../../state/graph/reducer";
import Spinner from "../../../components/Main/Spinner/Spinner";
import Alert from "../../../components/Main/Alert/Alert";
import Button from "../../../components/Main/Button/Button";
import "./style.scss";

interface ExploreMetricItemGraphProps {
  name: string,
  job: string,
  instance: string,
  rateEnabled: boolean,
  isBucket: boolean,
}

const ExploreMetricItem: FC<ExploreMetricItemGraphProps> = ({
  name,
  job,
  instance,
  rateEnabled,
  isBucket
}) => {
  const { customStep, yaxis } = useGraphState();
  const { period } = useTimeState();

  const graphDispatch = useGraphDispatch();
  const timeDispatch = useTimeDispatch();

  const [showAllSeries, setShowAllSeries] = useState(false);

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
      if (instance) {
        return `
label_map(
  histogram_quantiles("__name__", 0.5, 0.95, 0.99, sum(rate(${base})) by (vmrange, le)),
  "__name__",
  "0.5", "q50",
  "0.95", "q95",
  "0.99", "q99",
)`;
      }
      return `
with (q = histogram_quantile(0.95, sum(rate(${base})) by (instance, vmrange, le))) (
  alias(min(q), "q95min"),
  alias(max(q), "q95max"),
  alias(avg(q), "q95avg"),
)`;
    }
    if (instance) {
      const queryBase = rateEnabled ? `label_match(rollup_rate(${base}), "rollup", "max")` : `max_over_time(${base})`;
      return `alias(max(${queryBase}), "max")`;
    }
    const queryBase = rateEnabled ? `rollup_rate(${base})` : `rollup(${base})`;
    return `
with (q = ${queryBase}) (
  alias(min(label_match(q, "rollup", "min")), "min"),
  alias(max(label_match(q, "rollup", "max")), "max"),
  alias(avg(label_match(q, "rollup", "avg")), "avg"),
)`;
  }, [name, job, instance, rateEnabled, isBucket]);

  const { isLoading, graphData, error, warning } = useFetchQuery({
    predefinedQuery: [query],
    visible: true,
    customStep,
    showAllSeries
  });

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  const handleShowAll = () => {
    setShowAllSeries(true);
  };

  return (
    <div className="vm-explore-metrics-item-graph">
      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {warning && <Alert variant="warning">
        <div className="vm-explore-metrics-item-graph__warning">
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
          customStep={customStep}
          query={[query]}
          yaxis={yaxis}
          setYaxisLimits={setYaxisLimits}
          setPeriod={setPeriod}
        />
      )}
    </div>
  );
};

export default ExploreMetricItem;
