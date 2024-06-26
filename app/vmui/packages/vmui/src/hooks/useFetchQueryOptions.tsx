import React, { useEffect, useState, useRef, Dispatch, SetStateAction } from "preact/compat";
import { useAppState } from "../state/common/StateContext";
import { AutocompleteOptions } from "../components/Main/Autocomplete/Autocomplete";
import { LabelIcon, MetricIcon, ValueIcon } from "../components/Main/Icons";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCallback } from "react";
import debounce from "lodash.debounce";
import { useQueryDispatch, useQueryState } from "../state/query/QueryStateContext";
import { QueryContextType } from "../types";
import { AUTOCOMPLETE_LIMITS } from "../constants/queryAutocomplete";
import { escapeDoubleQuotes, escapeRegexp } from "../utils/regexp";
import dayjs from "dayjs";

enum TypeData {
  metric = "metric",
  label = "label",
  labelValue = "labelValue"
}

type FetchDataArgs = {
  value: string;
  urlSuffix: string;
  setter: Dispatch<SetStateAction<AutocompleteOptions[]>>
  type: TypeData;
  params?: URLSearchParams;
}

type FetchQueryArguments = {
  valueByContext: string;
  metric: string;
  label: string;
  context: QueryContextType
}

const icons = {
  [TypeData.metric]: <MetricIcon/>,
  [TypeData.label]: <LabelIcon/>,
  [TypeData.labelValue]: <ValueIcon/>,
};

export const useFetchQueryOptions = ({ valueByContext, metric, label, context }: FetchQueryArguments) => {
  const { serverUrl } = useAppState();
  const { period: { start, end } } = useTimeState();
  const { autocompleteCache } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const [loading, setLoading] = useState(false);
  const [value, setValue] = useState(valueByContext);
  const debouncedSetValue = debounce(setValue, 500);
  useEffect(() => {
    debouncedSetValue(valueByContext);
    return debouncedSetValue.cancel;
  }, [valueByContext, debouncedSetValue]);

  const [metrics, setMetrics] = useState<AutocompleteOptions[]>([]);
  const [labels, setLabels] = useState<AutocompleteOptions[]>([]);
  const [labelValues, setLabelValues] = useState<AutocompleteOptions[]>([]);

  const abortControllerRef = useRef(new AbortController());

  const getQueryParams = useCallback((params?: Record<string, string>) => {
    const startDay = dayjs(start * 1000).startOf("day").valueOf() / 1000;
    const endDay = dayjs(end * 1000).endOf("day").valueOf() / 1000;

    return new URLSearchParams({
      ...(params || {}),
      limit: `${AUTOCOMPLETE_LIMITS.queryLimit}`,
      start: `${startDay}`,
      end: `${endDay}`
    });
  }, [start, end]);

  const processData = (data: string[], type: TypeData) => {
    return data.map(l => ({
      value: l,
      type: `${type}`,
      icon: icons[type]
    }));
  };

  const fetchData = async ({ value, urlSuffix, setter, type, params }: FetchDataArgs) => {
    if (!value && type === TypeData.metric) return;
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;
    const key = {
      type,
      value,
      start: params?.get("start") || "",
      end: params?.get("end") || "",
      match: params?.get("match[]") || ""
    };
    setLoading(true);
    try {
      const cachedData = autocompleteCache.get(key);
      if (cachedData) {
        setter(processData(cachedData, type));
        setLoading(false);
        return;
      }
      const response = await fetch(`${serverUrl}/api/v1/${urlSuffix}?${params}`, { signal });
      if (response.ok) {
        const { data } = await response.json() as { data: string[] };
        setter(processData(data, type));
        queryDispatch({ type: "SET_AUTOCOMPLETE_CACHE", payload: { key, value: data } });
      }
      setLoading(false);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        queryDispatch({ type: "SET_AUTOCOMPLETE_CACHE", payload: { key, value: [] } });
        setLoading(false);
        console.error(e);
      }
    }
  };

  // fetch metrics
  useEffect(() => {
    const isInvalidContext = context !== QueryContextType.metricsql && context !== QueryContextType.empty;
    if (!serverUrl || !metric || isInvalidContext) {
      return;
    }
    setMetrics([]);

    const metricReEscaped = escapeDoubleQuotes(escapeRegexp(metric));

    fetchData({
      value,
      urlSuffix: "label/__name__/values",
      setter: setMetrics,
      type: TypeData.metric,
      params: getQueryParams({ "match[]": `{__name__=~".*${metricReEscaped}.*"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric]);

  // fetch labels
  useEffect(() => {
    if (!serverUrl || !metric || context !== QueryContextType.label) {
      return;
    }
    setLabels([]);

    const metricEscaped = escapeDoubleQuotes(metric);

    fetchData({
      value,
      urlSuffix: "labels",
      setter: setLabels,
      type: TypeData.label,
      params: getQueryParams({ "match[]": `{__name__="${metricEscaped}"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric]);

  // fetch labelValues
  useEffect(() => {
    if (!serverUrl || !metric || !label || context !== QueryContextType.labelValue) {
      return;
    }
    setLabelValues([]);

    const metricEscaped = escapeDoubleQuotes(metric);
    const valueReEscaped = escapeDoubleQuotes(escapeRegexp(value));

    fetchData({
      value,
      urlSuffix: `label/${label}/values`,
      setter: setLabelValues,
      type: TypeData.labelValue,
      params: getQueryParams({ "match[]": `{__name__="${metricEscaped}", ${label}=~".*${valueReEscaped}.*"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric, label]);

  return {
    metrics,
    labels,
    labelValues,
    loading,
  };
};
