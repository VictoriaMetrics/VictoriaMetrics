import { useAppState } from "../../../state/common/StateContext";
import { useState } from "react";
import { ErrorTypes } from "../../../types";
import { useSearchParams } from "react-router-dom";
import { getRetentionFiltersDebug } from "../../../api/retention-filters-debug";
import { useCallback } from "preact/compat";

export const useDebugRetentionFilters = () => {
  const { serverUrl } = useAppState();
  const [searchParams, setSearchParams] = useSearchParams();

  const [data, setData] = useState<Map<string, string>>(new Map());
  const [loading, setLoading] = useState(false);
  const [metricsError, setMetricsError] = useState<ErrorTypes | string>();
  const [flagsError, setFlagsError] = useState<ErrorTypes | string>();
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchData = useCallback(async (flags: string, metrics: string) => {
    metrics ? setMetricsError("") : setMetricsError("metrics are required");
    flags ? setFlagsError("") : setFlagsError("flags are required");
    if (!metrics || !flags) return;

    searchParams.set("flags", flags);
    searchParams.set("metrics", metrics);
    setSearchParams(searchParams);
    const fetchUrl = getRetentionFiltersDebug(serverUrl, flags, metrics);
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);

      const resp = await response.json();
      setData(new Map(Object.entries(resp.result || {})));
      setMetricsError(resp.error?.metrics || "");
      setFlagsError(resp.error?.flags || "");
      setError("");

    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setLoading(false);
  }, [serverUrl]);

  return {
    data,
    error: error,
    metricsError: metricsError,
    flagsError: flagsError,
    loading,
    applyFilters: fetchData
  };
};
