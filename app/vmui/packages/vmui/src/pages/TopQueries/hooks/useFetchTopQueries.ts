import { useEffect, useState } from "react";
import { ErrorTypes } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useMemo } from "preact/compat";
import { getTopQueries } from "../../../api/top-queries";
import { TopQueriesData } from "../../../types";
import { useTopQueriesState } from "../../../state/topQueries/TopQueriesStateContext";

export const useFetchTopQueries = () => {
  const { serverUrl } = useAppState();
  const { topN, maxLifetime, runQuery } = useTopQueriesState();

  const [data, setData] = useState<TopQueriesData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getTopQueries(serverUrl, topN, maxLifetime), [serverUrl, topN, maxLifetime]);

  const fetchData = async () => {
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();
      if (response.ok) {
        const list = ["topByAvgDuration", "topByCount", "topBySumDuration"] as (keyof TopQueriesData)[];
        list.forEach(key => {
          const target = resp[key];
          if (Array.isArray(target)) {
            target.forEach(t => t.timeRangeHours = +(t.timeRangeSeconds/3600).toFixed(2));
          }
        });
      }

      setData(response.ok ? resp : null);
      setError(String(resp.error || ""));
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchData();
  }, [runQuery]);

  return {
    data,
    error,
    loading
  };
};
