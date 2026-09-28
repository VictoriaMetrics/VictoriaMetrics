import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { compactObject } from "../../../utils/object";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { getInitialTimeState } from "../../../state/time/reducer";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import useEventListener from "../../../hooks/useEventListener";

export const useSetQueryParams = () => {
  const { duration, relativeTime, period: { date } } = useTimeState();
  const timeDispatch = useTimeDispatch();
  const { customStep } = useGraphState();
  const graphDispatch = useGraphDispatch();
  const [searchParams] = useSearchParams();
  const [isPopstate, setIsPopstate] = useState(false);
  const skipNextUrlUpdate = useRef(false);
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const setSearchParamsFromState = () => {
    // Preserve the URL until navigation has been applied to the state.
    if (isPopstate) return;
    if (skipNextUrlUpdate.current) {
      skipNextUrlUpdate.current = false;
      return;
    }

    const params = compactObject({
      ["g0.range_input"]: duration,
      ["g0.end_input"]: date,
      ["g0.step_input"]: customStep,
      ["g0.relative_time"]: relativeTime
    });

    setSearchParamsFromKeys(params);
  };

  useEffect(setSearchParamsFromState, [duration, relativeTime, date, customStep, isPopstate]);

  useEffect(() => {
    if (!isPopstate) return;

    const timeFromUrl = getInitialTimeState();
    const isDurationDifferent = timeFromUrl.duration !== duration;
    const isRelativeTimeDifferent = timeFromUrl.relativeTime !== relativeTime;
    const isDateDifferent = timeFromUrl.relativeTime === "none" && timeFromUrl.period.date !== date;
    const someNotEqual = isDurationDifferent || isRelativeTimeDifferent || isDateDifferent;
    if (someNotEqual) {
      timeDispatch({ type: "SET_TIME_STATE", payload: timeFromUrl });
    }

    // Apply the URL step after the time range's automatic step update.
    // Keep state-to-URL synchronization paused until both have been restored.
    const timer = setTimeout(() => {
      const customStepFromUrl = searchParams.get("g0.step_input") || timeFromUrl.period.step;
      if (customStepFromUrl && customStepFromUrl !== customStep) {
        graphDispatch({ type: "SET_CUSTOM_STEP", payload: customStepFromUrl });
      }
      // Restoring a relative range recalculates its end time. Do not push a new
      // history entry for that update, otherwise Forward navigation is lost.
      skipNextUrlUpdate.current = true;
      setIsPopstate(false);
    }, 50);

    return () => clearTimeout(timer);
  }, [searchParams, isPopstate, customStep]);

  useEventListener("popstate", () => {
    setIsPopstate(true);
  });
};
