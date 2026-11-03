import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface queryProps {
  job: string
  instance?: string
  metrics: string
  size: string
}

export const useSetQueryParams = ({ job, instance, metrics, size }: queryProps) => {
  const { duration, relativeTime, period: { date } } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.relative_time"]: relativeTime,
      size,
      job,
      instance,
      metrics
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, job, instance, metrics, size]);
  useEffect(setSearchParamsFromState, []);
};
