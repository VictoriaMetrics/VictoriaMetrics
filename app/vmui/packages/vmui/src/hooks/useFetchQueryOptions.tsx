import React, { useEffect, useState, useRef } from "preact/compat";
import { StateUpdater } from "preact/hooks";
import { useAppState } from "../state/common/StateContext";
import { AutocompleteOptions } from "../components/Main/Autocomplete/Autocomplete";
import { LabelIcon, MetricIcon, ValueIcon } from "../components/Main/Icons";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCallback } from "react";
import qs from "qs";
import dayjs from "dayjs";

enum TypeData {
  metric,
  label,
  value
}

type FetchDataArgs = {
  urlSuffix: string;
  setter: StateUpdater<AutocompleteOptions[]>;
  type: TypeData;
  params?: URLSearchParams;
}

const icons = {
  [TypeData.metric]: <MetricIcon />,
  [TypeData.label]: <LabelIcon />,
  [TypeData.value]: <ValueIcon />,
};

const QUERY_LIMIT = 1000;

export const useFetchQueryOptions = ({ metric, label }: { metric: string; label: string }) => {
  const { serverUrl } = useAppState();
  const { period: { start, end } } = useTimeState();

  const [metrics, setMetrics] = useState<AutocompleteOptions[]>([]);
  const [labels, setLabels] = useState<AutocompleteOptions[]>([]);
  const [values, setValues] = useState<AutocompleteOptions[]>([]);

  const prevParams = useRef<Record<string, URLSearchParams>>({});

  const getQueryParams = useCallback((params?: Record<string, string>) => {
    const roundedStart = dayjs(start).startOf("day").valueOf();
    const roundedEnd = dayjs(end).endOf("day").valueOf();

    return new URLSearchParams({
      ...(params || {}),
      limit: `${QUERY_LIMIT}`,
      start: `${roundedStart}`,
      end: `${roundedEnd}`
    });
  }, [start, end]);

  const isParamsEqual = (prev: URLSearchParams, next: URLSearchParams) => {
    const queryNext = qs.parse(next.toString());
    const queryPrev = qs.parse(prev.toString());
    return JSON.stringify(queryPrev) === JSON.stringify(queryNext);
  };

  const fetchData = async ({ urlSuffix, setter, type, params }: FetchDataArgs) => {
    try {
      const response = await fetch(`${serverUrl}/api/v1/${urlSuffix}?${params}`);
      if (response.ok) {
        const { data } = await response.json() as { data: string[] };
        setter(data.map(l => ({
          value: l,
          type: `${type}`,
          icon: icons[type]
        })));
      }
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (!serverUrl) {
      setMetrics([]);
      return;
    }

    const params = getQueryParams();
    const prev = prevParams.current.metrics || new URLSearchParams({});
    if (isParamsEqual(params, prev)) return;

    fetchData({
      urlSuffix: "label/__name__/values",
      setter: setMetrics,
      type: TypeData.metric,
      params
    });

    prevParams.current = { ...prevParams.current, metrics: params };
  }, [serverUrl, getQueryParams]);

  useEffect(() => {
    const notFoundMetric = !metrics.find(m => m.value === metric);
    if (!serverUrl || notFoundMetric) {
      setLabels([]);
      return;
    }

    const params = getQueryParams({ "match[]": metric });
    const prev = prevParams.current.labels || new URLSearchParams({});
    if (isParamsEqual(params, prev)) return;

    fetchData({
      urlSuffix: "labels",
      setter: setLabels,
      type: TypeData.label,
      params
    });

    prevParams.current = { ...prevParams.current, labels: params };
  }, [serverUrl, metric, getQueryParams]);

  useEffect(() => {
    const notFoundMetric = !metrics.find(m => m.value === metric);
    const notFoundLabel = !labels.find(l => l.value === label);
    if (!serverUrl || notFoundMetric || notFoundLabel) {
      setValues([]);
      return;
    }

    const params = getQueryParams({ "match[]": metric });
    const prev = prevParams.current.values || new URLSearchParams({});
    if (isParamsEqual(params, prev)) return;

    fetchData({
      urlSuffix: `label/${label}/values`,
      setter: setValues,
      type: TypeData.value,
      params
    });

    prevParams.current = { ...prevParams.current, values: params };
  }, [serverUrl, metric, label, getQueryParams]);

  return {
    metrics,
    labels,
    values,
  };
};
