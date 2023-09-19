import { useEffect, useState } from "preact/compat";
import { useAppState } from "../state/common/StateContext";

export const useFetchQueryOptions = ({ metric, label }: { metric: string; label: string }) => {
  const { serverUrl } = useAppState();

  const [metricNames, setMetricNames] = useState<string[]>([]);
  const [labels, setLabels] = useState<string[]>([]);
  const [values, setValues] = useState<string[]>([]);

  useEffect(() => {
    const fetchMetrics = async () => {

      try {
        const response = await fetch(`${serverUrl}/api/v1/label/__name__/values`);
        if (response.ok) {
          const { data } = await response.json();
          setMetricNames(data);
        }
      } catch (e) {
        console.error(e);
      }
    };

    if (!serverUrl) return;
    fetchMetrics();
  }, [serverUrl]);

  useEffect(() => {
    const fetchLabels = async () => {
      try {
        const response = await fetch(`${serverUrl}/api/v1/labels?match[]=${metric}`);
        if (response.ok) {
          const { data } = await response.json();
          setLabels(data);
        }
      } catch (e) {
        console.error(e);
      }
    };

    if (!serverUrl || !metric) return;
    fetchLabels();
  }, [serverUrl, metric]);

  useEffect(() => {
    const fetchValues = async () => {
      try {
        const response = await fetch(`${serverUrl}/api/v1/label/${label}/values?match[]=${metric}`);
        if (response.ok) {
          const { data } = await response.json();
          setValues(data);
        }
      } catch (e) {
        console.error(e);
      }
    };

    if (!serverUrl || !metric || !label) return;
    fetchValues();
  }, [serverUrl, metric, label]);

  return {
    metricNames,
    labels,
    values,
  };
};
