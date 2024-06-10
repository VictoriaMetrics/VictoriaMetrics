import { useEffect, useState } from "react";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import { displayTypeTabs } from "../DisplayTypeSwitch";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import { useSearchParams } from "react-router-dom";
import { useCallback } from "preact/compat";
import { getInitialDisplayType } from "../../../state/customPanel/reducer";
import { getInitialTimeState } from "../../../state/time/reducer";
import useEventListener from "../../../hooks/useEventListener";
import { getQueryArray } from "../../../utils/query-string";
import { arrayEquals } from "../../../utils/array";
import { isEqualURLSearchParams } from "../../../utils/url";

export const useSetQueryParams = () => {
  const { tenantId } = useAppState();
  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();
  const { duration, relativeTime, period: { date, step } } = useTimeState();
  const { customStep } = useGraphState();
  const [searchParams, setSearchParams] = useSearchParams();

  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();
  const graphDispatch = useGraphDispatch();
  const queryDispatch = useQueryDispatch();
  const customPanelDispatch = useCustomPanelDispatch();

  const [isPopstate, setIsPopstate] = useState(false);

  const setterSearchParams = useCallback(() => {
    if (isPopstate) {
      // After the popstate event, the states synchronizes with the searchParams,
      // so there's no need to refresh the searchParams again.
      setIsPopstate(false);
      return;
    }

    const newSearchParams = new URLSearchParams(searchParams);

    query.forEach((q, i) => {
      const group = `g${i}`;
      if ((searchParams.get(`${group}.expr`) !== q) && q) {
        newSearchParams.set(`${group}.expr`, q);
      }

      if (searchParams.get(`${group}.range_input`) !== duration) {
        newSearchParams.set(`${group}.range_input`, duration);
      }

      if (searchParams.get(`${group}.end_input`) !== date) {
        newSearchParams.set(`${group}.end_input`, date);
      }

      if (searchParams.get(`${group}.relative_time`) !== relativeTime) {
        newSearchParams.set(`${group}.relative_time`, relativeTime || "none");
      }

      const stepFromUrl = searchParams.get(`${group}.step_input`) || step;
      if (stepFromUrl && (stepFromUrl !== customStep)) {
        newSearchParams.set(`${group}.step_input`, customStep);
      }

      const displayTypeCode = `${displayTypeTabs.find(t => t.value === displayType)?.prometheusCode || 0}`;
      if (searchParams.get(`${group}.tab`) !== displayTypeCode) {
        newSearchParams.set(`${group}.tab`, `${displayTypeCode}`);
      }

      if (searchParams.get(`${group}.tenantID`) !== tenantId && tenantId) {
        newSearchParams.set(`${group}.tenantID`, tenantId);
      }
    });
    if (isEqualURLSearchParams(newSearchParams, searchParams) || !newSearchParams.size) return;
    setSearchParams(newSearchParams);
  }, [tenantId, displayType, query, duration, relativeTime, date, step, customStep]);

  useEffect(() => {
    const timer = setTimeout(setterSearchParams, 200);
    return () => clearTimeout(timer);
  }, [setterSearchParams]);

  useEffect(() => {
    // Synchronize the states with searchParams only after the popstate event.
    if (!isPopstate) return;

    const timeFromUrl = getInitialTimeState();
    const isDurationDifferent = (timeFromUrl.duration !== duration);
    const isRelativeTimeDifferent = timeFromUrl.relativeTime !== relativeTime;
    const isDateDifferent = timeFromUrl.relativeTime === "none" && timeFromUrl.period.date !== date;
    const someNotEqual = isDurationDifferent || isRelativeTimeDifferent || isDateDifferent;
    if (someNotEqual) {
      timeDispatch({ type: "SET_TIME_STATE", payload: timeFromUrl });
    }

    const displayTypeFromUrl = getInitialDisplayType();
    if (displayTypeFromUrl !== displayType) {
      customPanelDispatch({ type: "SET_DISPLAY_TYPE", payload: displayTypeFromUrl });
    }

    const tenantIdFromUrl = searchParams.get("g0.tenantID") || "";
    if (tenantIdFromUrl !== tenantId) {
      dispatch({ type: "SET_TENANT_ID", payload: tenantIdFromUrl });
    }

    const queryFromUrl = getQueryArray();
    if (!arrayEquals(queryFromUrl, query)) {
      queryDispatch({ type: "SET_QUERY", payload: queryFromUrl });
      timeDispatch({ type: "RUN_QUERY" });
    }

    // Timer prevents customStep reset on time range change.
    const timer = setTimeout(() => {
      const customStepFromUrl = searchParams.get("g0.step_input") || step;
      if (customStepFromUrl && customStepFromUrl !== customStep) {
        graphDispatch({ type: "SET_CUSTOM_STEP", payload: customStepFromUrl });
      }
    }, 50);

    return () => clearTimeout(timer);
  }, [searchParams, isPopstate]);

  useEventListener("popstate", () => {
    setIsPopstate(true);
  });
};
