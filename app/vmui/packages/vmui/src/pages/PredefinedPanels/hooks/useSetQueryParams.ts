import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";

export const useSetQueryParams = () => {
  const { duration, relativeTime, period: { date, step } } = useTimeState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: step,
      ["g0.relative_time"]: relativeTime
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, step]);
  useEffect(setSearchParamsFromState, []);
};
