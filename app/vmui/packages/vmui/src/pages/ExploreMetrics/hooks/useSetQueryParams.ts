import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";

interface queryProps {
  job: string
  instance?: string
  metrics: string
  size: string
}

export const useSetQueryParams = ({ job, instance, metrics, size }: queryProps) => {
  const { duration, relativeTime, period: { date, step } } = useTimeState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: step,
      ["g0.relative_time"]: relativeTime,
      size,
      job,
      instance,
      metrics
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, step, job, instance, metrics, size]);
  useEffect(setSearchParamsFromState, []);
};
