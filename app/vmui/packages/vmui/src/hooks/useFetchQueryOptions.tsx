import React, { useEffect, useState, useRef } from "preact/compat";
import { StateUpdater } from "preact/hooks";
import { useAppState } from "../state/common/StateContext";
import { AutocompleteOptions } from "../components/Main/Autocomplete/Autocomplete";
import { LabelIcon, MetricIcon, ValueIcon } from "../components/Main/Icons";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCallback } from "react";
import debounce from "lodash.debounce";
import { useQueryDispatch, useQueryState } from "../state/query/QueryStateContext";
import { QueryContextType } from "../types";
import { AUTOCOMPLETE_LIMITS } from "../constants/queryAutocomplete";

enum TypeData {
  metric = "metric",
  label = "label",
  value = "value"
}

type FetchDataArgs = {
  value: string;
  urlSuffix: string;
  setter: StateUpdater<AutocompleteOptions[]>;
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
  [TypeData.value]: <ValueIcon/>,
};

export const useFetchQueryOptions = ({ valueByContext, metric, label, context }: FetchQueryArguments) => {
  const { serverUrl } = useAppState();
  const { period: { start, end } } = useTimeState();
  const { autocompleteCache } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const [loading, setLoading] = useState(false);
  const [value, setValue] = useState(valueByContext);
  const debouncedSetValue = debounce(setValue, 800);
  useEffect(() => {
    debouncedSetValue(valueByContext);
    return debouncedSetValue.cancel;
  }, [valueByContext, debouncedSetValue]);

  const [metrics, setMetrics] = useState<AutocompleteOptions[]>([]);
  const [labels, setLabels] = useState<AutocompleteOptions[]>([]);
  const [values, setValues] = useState<AutocompleteOptions[]>([]);

  const abortControllerRef = useRef(new AbortController());

  const getQueryParams = useCallback((params?: Record<string, string>) => {
    return new URLSearchParams({
      ...(params || {}),
      limit: `${AUTOCOMPLETE_LIMITS.queryLimit}`,
      start: `${start}`,
      end: `${end}`
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
        return;
      }
      const response = await fetch(`${serverUrl}/api/v1/${urlSuffix}?${params}`, { signal });
      if (response.ok) {
        const { data } = await response.json() as { data: string[] };
        setter(processData(data, type));
        queryDispatch({ type: "SET_AUTOCOMPLETE_CACHE", payload: { key, value: data } });
      }
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        queryDispatch({ type: "SET_AUTOCOMPLETE_CACHE", payload: { key, value: [] } });
        console.error(e);
      }
    } finally {
      setLoading(false);
    }
  };

  // fetch metrics
  useEffect(() => {
    const isInvalidContext = context !== QueryContextType.metricsql && context !== QueryContextType.empty;
    if (!serverUrl || !metric || isInvalidContext) {
      return;
    }
    setMetrics([]);

    fetchData({
      value,
      urlSuffix: "label/__name__/values",
      setter: setMetrics,
      type: TypeData.metric,
      params: getQueryParams({ "match[]": `{__name__=~".*${metric}.*"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric]);

  // fetch labels
  useEffect(() => {
    if (!serverUrl || !metric || context !== QueryContextType.label) {
      return;
    }
    setLabels([]);

    fetchData({
      value,
      urlSuffix: "labels",
      setter: setLabels,
      type: TypeData.label,
      params: getQueryParams({ "match[]": `{__name__=~".*${metric}.*"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric]);

  // fetch values
  useEffect(() => {
    if (!serverUrl || !metric || !label || context !== QueryContextType.value) {
      return;
    }
    setValues([]);

    fetchData({
      value,
      urlSuffix: `label/${label}/values`,
      setter: setValues,
      type: TypeData.value,
      params: getQueryParams({ "match[]": `{__name__=~".*${metric}.*", ${label}=~".*${value}.*"}` })
    });

    return () => abortControllerRef.current?.abort();
  }, [serverUrl, value, context, metric, label]);

  return {
    metrics,
    labels,
    values,
    loading,
  };
};
