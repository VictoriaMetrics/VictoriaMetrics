import { ErrorTypes, TopQuery } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useMemo, useState } from "preact/compat";
import { getTopQueries } from "../../../api/top-queries";
import { TopQueriesData } from "../../../types";
import { getDurationFromMilliseconds, relativeTimeOptions } from "../../../utils/time";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import router from "../../../router";

interface useFetchTopQueriesProps {
  topN: number;
  maxLifetime: string;
}

const getQueryUrl = (row: TopQuery, timeRange: string) => {
  const { query, timeRangeSeconds } = row;
  const params = [`g0.expr=${encodeURIComponent(query)}`];
  const relativeTimeId = relativeTimeOptions.find(t => t.duration === timeRange)?.id;
  if (relativeTimeId) {
    params.push(`g0.relative_time=${relativeTimeId}`);
  }
  if (timeRangeSeconds) {
    params.push(`g0.range_input=${timeRange}`);
  }
  return `${router.home}?${params.join("&")}`;
};

const processResponse = (data: TopQueriesData) => {
  const list = ["topByAvgDuration", "topByCount", "topBySumDuration"] as (keyof TopQueriesData)[];

  list.forEach(key => {
    const target = data[key] as TopQuery[];
    if (!Array.isArray(target)) return;

    target.forEach(t => {
      const timeRange = getDurationFromMilliseconds(t.timeRangeSeconds*1000);
      t.url = getQueryUrl(t, timeRange);
      t.timeRange = timeRange;
    });
  });

  return data;
};

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
      setData(response.ok ? processResponse(resp) : null);
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
