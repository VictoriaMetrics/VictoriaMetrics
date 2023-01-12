import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";
import { useGraphState } from "../../../state/graph/GraphStateContext";

export const useSetQueryParams = () => {
  const { duration, relativeTime, period: { date } } = useTimeState();
  const { customStep } = useGraphState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: customStep,
      ["g0.relative_time"]: relativeTime
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, customStep]);
  useEffect(setSearchParamsFromState, []);
};
