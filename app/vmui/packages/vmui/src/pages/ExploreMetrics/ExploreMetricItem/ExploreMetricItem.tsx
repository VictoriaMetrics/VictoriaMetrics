import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import Accordion from "../../../components/Main/Accordion/Accordion";
import { useFetchQuery } from "../../../hooks/useFetchQuery";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import GraphView from "../../../components/Views/GraphView/GraphView";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { AxisRange } from "../../../state/graph/reducer";
import Spinner from "../../../components/Main/Spinner/Spinner";
import Alert from "../../../components/Main/Alert/Alert";
import Button from "../../../components/Main/Button/Button";
import "./style.scss";

interface ExploreMetricItemProps {
  name: string,
  job: string,
  instance: string
}

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({ name, job, instance }) => {
  const { customStep, yaxis } = useGraphState();
  const { period } = useTimeState();

  const graphDispatch = useGraphDispatch();
  const timeDispatch = useTimeDispatch();

  const [isOpen, setIsOpen] = useState(false);
  const [showAllSeries, setShowAllSeries] = useState(false);

  const query = useMemo(() => {
    const params = Object.entries({ job, instance }).filter(val => val[1]).map(([key, val]) => `${key}="${val}"`);

    const fullQuery = `${name}{${params.join(",")}}`;
    const counterQuery = `rate(${fullQuery})`;
    const withoutQuery = `sum(${counterQuery}) without (job)`;

    const isCounter = /_sum?|_total?|_count?/.test(name);
    const isWithout = isCounter && job && !instance;

    if (isCounter) return counterQuery;
    if (isWithout) return withoutQuery;
    return fullQuery;
  }, [name, job, instance]);

  const { isLoading, graphData, error, warning } = useFetchQuery({
    predefinedQuery: [query],
    visible: isOpen,
    customStep,
    showAllSeries
  });

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  const handleToggleOpen = (val: boolean) => {
    setIsOpen(val);
  };

  const handleShowAll = () => {
    setShowAllSeries(true);
  };

  useEffect(() => {
    if (isOpen) {
      timeDispatch({ type: "RUN_QUERY" });
    }
  }, [query]);

  return (
    <div className="vm-explore-metrics-item">
      <Accordion
        onChange={handleToggleOpen}
        title={<div className="vm-explore-metrics-item__header">{name}</div>}
      >
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
      </Accordion>
    </div>
  );
};

export default ExploreMetricItem;
