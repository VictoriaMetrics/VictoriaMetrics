import React, { FC, useEffect, useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import StepConfigurator from "../../Configurators/StepConfigurator/StepConfigurator";
import "./style.scss";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import usePrevious from "../../../hooks/usePrevious";

interface ExploreMetricsHeaderProps {
  jobs: string[]
  instances: string[]
  names: string[]
  job: string
  instance: string
  selectedMetrics: string[]
  onChangeJob: (job: string) => void
  onChangeInstance: (instance: string) => void
  onToggleMetric: (name: string) => void
}

const ExploreMetricsHeader: FC<ExploreMetricsHeaderProps> = ({
  jobs,
  instances,
  names,
  job,
  instance,
  selectedMetrics,
  onChangeJob,
  onChangeInstance,
  onToggleMetric
}) => {

  const { period: { step }, duration } = useTimeState();
  const { customStep } = useGraphState();
  const graphDispatch = useGraphDispatch();
  const prevDuration = usePrevious(duration);

  const noInstanceText = useMemo(() => job ? "" : "No instances. Please select job", [job]);
  const noMetricsText = useMemo(() => job ? "" : "No metric names. Please select job", [job]);

  const handleChangeStep = (value: string) => {
    graphDispatch({ type: "SET_CUSTOM_STEP", payload: value });
  };

  useEffect(() => {
    if (duration === prevDuration || !prevDuration) return;
    if (customStep) handleChangeStep(step || "1s");
  }, [duration, prevDuration]);

  useEffect(() => {
    if (!customStep && step) handleChangeStep(step);
  }, [step]);

  return (
    <div className="vm-explore-metrics-header vm-block">
      <div className="vm-explore-metrics-header__job">
        <Select
          value={job}
          list={jobs}
          label="Job"
          placeholder="Please select job"
          onChange={onChangeJob}
          autofocus
        />
      </div>
      <div className="vm-explore-metrics-header__instance">
        <Select
          value={instance}
          list={instances}
          label="Instance"
          placeholder="Please select instance"
          onChange={onChangeInstance}
          noOptionsText={noInstanceText}
          clearable
        />
      </div>
      <div className="vm-explore-metrics-header__step">
        <StepConfigurator
          defaultStep={step}
          setStep={handleChangeStep}
          value={customStep}
        />
      </div>
      <div className="vm-explore-metrics-header-metrics">
        <Select
          value={selectedMetrics}
          list={names}
          placeholder="Search metric name"
          onChange={onToggleMetric}
          noOptionsText={noMetricsText}
          clearable
        />
      </div>
    </div>
  );
};

export default ExploreMetricsHeader;
