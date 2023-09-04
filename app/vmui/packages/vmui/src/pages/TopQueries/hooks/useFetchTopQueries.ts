import { ErrorTypes } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useMemo, useState } from "preact/compat";
import { getTopQueries } from "../../../api/top-queries";
import { TopQueriesData } from "../../../types";
import { getDurationFromMilliseconds } from "../../../utils/time";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

interface useFetchTopQueriesProps {
  topN: number;
  maxLifetime: string;
}

export const useFetchTopQueries = ({ topN, maxLifetime }: useFetchTopQueriesProps) => {
  const { serverUrl } = useAppState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [data, setData] = useState<TopQueriesData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getTopQueries(serverUrl, topN, maxLifetime), [serverUrl, topN, maxLifetime]);

  const fetchData = async () => {
    setLoading(true);
    setSearchParamsFromKeys({ topN, maxLifetime });
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();
      if (response.ok) {
        const list = ["topByAvgDuration", "topByCount", "topBySumDuration"] as (keyof TopQueriesData)[];
        list.forEach(key => {
          const target = resp[key];
          if (Array.isArray(target)) {
            target.forEach(t => t.timeRange = getDurationFromMilliseconds(t.timeRangeSeconds*1000));
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

  return {
    data,
    error,
    loading,
    fetch: fetchData
  };
};
