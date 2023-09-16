import { useEffect, useState } from "preact/compat";
import { useAppState } from "../state/common/StateContext";

export const useFetchQueryOptions = ({ metric }: {metric: string}) => {
  const { serverUrl } = useAppState();

  const [metricNames, setMetricNames] = useState<string[]>([]);
  const [labels, setLabels] = useState<string[]>([]);

  useEffect(() => {
    const fetchMetrics = async () => {

      try {
        const response = await fetch(`${serverUrl}/api/v1/label/__name__/values`);
        // const values = await fetch(`${serverUrl}/api/v1/label/device/values?match[]=node_arp_entries`);
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

  return {
    metricNames,
    labels
  };
};
