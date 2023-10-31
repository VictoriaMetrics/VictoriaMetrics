import React, { useEffect, useState } from "preact/compat";
import { StateUpdater } from "preact/hooks";
import { useAppState } from "../state/common/StateContext";
import { AutocompleteOptions } from "../components/Main/Autocomplete/Autocomplete";
import { LabelIcon, MetricIcon, ValueIcon } from "../components/Main/Icons";

enum TypeData {
  metric,
  label,
  value
}

type FetchDataArgs = {
  url: string;
  setter: StateUpdater<AutocompleteOptions[]>;
  type: TypeData;
}

const icons = {
  [TypeData.metric]: <MetricIcon />,
  [TypeData.label]: <LabelIcon />,
  [TypeData.value]: <ValueIcon />,
};

export const useFetchQueryOptions = ({ metric, label }: { metric: string; label: string }) => {
  const { serverUrl } = useAppState();

  const [metrics, setMetrics] = useState<AutocompleteOptions[]>([]);
  const [labels, setLabels] = useState<AutocompleteOptions[]>([]);
  const [values, setValues] = useState<AutocompleteOptions[]>([]);

  const fetchData = async ({ url, setter, type, }: FetchDataArgs) => {
    try {
      const response = await fetch(url);
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

    fetchData({
      url: `${serverUrl}/api/v1/label/__name__/values`,
      setter: setMetrics,
      type: TypeData.metric
    });
  }, [serverUrl]);

  useEffect(() => {
    const notFoundMetric = !metrics.find(m => m.value === metric);
    if (!serverUrl || notFoundMetric) {
      setLabels([]);
      return;
    }

    fetchData({
      url: `${serverUrl}/api/v1/labels?match[]=${metric}`,
      setter: setLabels,
      type: TypeData.label
    });
  }, [serverUrl, metric]);

  useEffect(() => {
    const notFoundMetric = !metrics.find(m => m.value === metric);
    const notFoundLabel = !labels.find(l => l.value === label);
    if (!serverUrl || notFoundMetric || notFoundLabel) {
      setValues([]);
      return;
    }

    fetchData({
      url: `${serverUrl}/api/v1/label/${label}/values?match[]=${metric}`,
      setter: setValues,
      type: TypeData.value
    });
  }, [serverUrl, metric, label]);

  return {
    metrics,
    labels,
    values,
  };
};
