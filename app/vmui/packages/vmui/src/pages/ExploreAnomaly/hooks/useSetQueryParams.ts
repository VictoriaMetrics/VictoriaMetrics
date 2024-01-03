import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphState } from "../../../state/graph/GraphStateContext";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface stateParams extends Record<string, string> {
  alias: string;
}

export const useSetQueryParams = ({ alias, ...args }: stateParams) => {
  const { duration, relativeTime, period: { date } } = useTimeState();
  const { customStep } = useGraphState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: customStep,
      ["g0.relative_time"]: relativeTime,
      alias,
      ...args,
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, customStep, alias, args]);
  useEffect(setSearchParamsFromState, []);
};
