import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";

export const useSetQueryParams = () => {
  const { duration, relativeTime, period: { date, step } } = useTimeState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      range_input: duration,
      end_input: date,
      step_input: step,
      relative_time: relativeTime
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, step]);
  useEffect(setSearchParamsFromState, []);
};
