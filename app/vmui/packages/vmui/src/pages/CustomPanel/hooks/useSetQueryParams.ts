import { useEffect, useState } from "react";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import { displayTypeTabs } from "../DisplayTypeSwitch";
import { useSearchParams } from "react-router-dom";
import { useCallback } from "preact/compat";
import { getInitialDisplayType } from "../../../state/customPanel/reducer";
import { getInitialTimeState } from "../../../state/time/reducer";
import useEventListener from "../../../hooks/useEventListener";
import { getQueryArray } from "../../../utils/query-string";
import { arrayEquals } from "../../../utils/array";
import { isEqualURLSearchParams } from "../../../utils/url";

export const useSetQueryParams = () => {
  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();
  const { duration, relativeTime, period: { date } } = useTimeState();
  const [searchParams, setSearchParams] = useSearchParams();

  const timeDispatch = useTimeDispatch();
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

      const exprHide = searchParams.get("expr.hide") || "";
      if (exprHide !== "") {
        newSearchParams.set("expr.hide", exprHide);
      }

      const displayTypeCode = `${displayTypeTabs.find(t => t.value === displayType)?.prometheusCode || 0}`;
      if (searchParams.get(`${group}.tab`) !== displayTypeCode) {
        newSearchParams.set(`${group}.tab`, `${displayTypeCode}`);
      }
    });

    // Remove extra parameters that exceed the request size
    const maxIndex = query.length - 1;
    Array.from(newSearchParams.keys()).forEach(key => {
      const match = key.match(/^g(\d+)\./);
      if (match && parseInt(match[1], 10) > maxIndex) {
        newSearchParams.delete(key);
      }
    });

    if (isEqualURLSearchParams(newSearchParams, searchParams) || !newSearchParams.size) return;
    setSearchParams(newSearchParams);
  }, [displayType, query, duration, relativeTime, date, searchParams, isPopstate]);

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

    const queryFromUrl = getQueryArray();
    if (!arrayEquals(queryFromUrl, query)) {
      queryDispatch({ type: "SET_QUERY", payload: queryFromUrl });
      timeDispatch({ type: "RUN_QUERY" });
    }

  }, [searchParams, isPopstate]);

  useEventListener("popstate", () => {
    setIsPopstate(true);
  });
};
