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
  isCounter: boolean,
  isBucket: boolean,
}

const ExploreMetricItem: FC<ExploreMetricItemGraphProps> = ({
  name,
  job,
  instance,
  rateEnabled,
  isCounter,
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
      .map(([key, val]) => `${key}="${val}"`);

    const base = `${name}{${params.join(",")}}`;
    const queryBase = rateEnabled ? `rate(${base})` : base;

    const queryBucket = `histogram_quantiles("quantile", 0.5, 0.99, increase(${base}[5m]))`;
    const queryBucketWithoutInstance = `histogram_quantiles("quantile", 0.5, 0.99, sum(increase(${base}[5m])) without (instance))`;
    const queryCounterWithoutInstance = `sum(${queryBase}) without (job)`;
    const queryWithoutInstance = `sum(${queryBase}) without (instance)`;

    const isCounterWithoutInstance = isCounter && job && !instance;
    const isBucketWithoutInstance = isBucket && job && !instance;
    const isWithoutInstance = !isCounter && job && !instance;

    if (isCounterWithoutInstance) return queryCounterWithoutInstance;
    if (isBucketWithoutInstance) return queryBucketWithoutInstance;
    if (isBucket) return queryBucket;
    if (isWithoutInstance) return queryWithoutInstance;
    return queryBase;
  }, [name, job, instance, rateEnabled, isCounter, isBucket]);

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
