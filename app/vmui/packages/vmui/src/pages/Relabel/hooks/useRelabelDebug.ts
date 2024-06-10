import { useAppState } from "../../../state/common/StateContext";
import { useState } from "react";
import { ErrorTypes, RelabelData } from "../../../types";
import { getMetricRelabelDebug } from "../../../api/metric-relabel";

export const useRelabelDebug = () => {
  const { serverUrl } = useAppState();

  const [data, setData] = useState<RelabelData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchData = async (config: string, metric: string) => {
    const fetchUrl = getMetricRelabelDebug(serverUrl, config, metric);
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();

      setData(resp.error ? null : resp);
      setError(String(resp.error || ""));
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setLoading(false);
  };

  return {
    data,
    error,
    loading,
    fetchData
  };
};
