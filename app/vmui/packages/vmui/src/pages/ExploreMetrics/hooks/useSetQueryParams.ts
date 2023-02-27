import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphState } from "../../../state/graph/GraphStateContext";
import { useSearchParams } from "react-router-dom";

interface queryProps {
  job: string
  instance?: string
  metrics: string
  size: string
}

export const useSetQueryParams = ({ job, instance, metrics, size }: queryProps) => {
  const { duration, relativeTime, period: { date } } = useTimeState();
  const { customStep } = useGraphState();
  const [, setSearchParams] = useSearchParams();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: customStep,
      ["g0.relative_time"]: relativeTime,
      size,
      job,
      instance,
      metrics
    });

    setSearchParams(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, customStep, job, instance, metrics, size]);
  useEffect(setSearchParamsFromState, []);
};
