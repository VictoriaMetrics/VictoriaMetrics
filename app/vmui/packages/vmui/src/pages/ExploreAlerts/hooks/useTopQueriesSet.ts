import { useEffect, useMemo, useState } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import { getTopQueries } from "../../../api/top-queries";
import { TopQueriesData } from "../../../types";

const TOP_N = 100;
const MAX_LIFETIME = "10m";

const normalizeQuery = (q: string): string => q.replace(/\s+/g, " ").trim();

export const useTopQueriesSet = (): Set<string> => {
  const { serverUrl } = useAppState();
  const [querySet, setQuerySet] = useState<Set<string>>(new Set());

  const fetchUrl = useMemo(() => getTopQueries(serverUrl, TOP_N, MAX_LIFETIME), [serverUrl]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch(fetchUrl);
        if (!response.ok) return;
        const data: TopQueriesData = await response.json();
        const queries = new Set<string>();
        const lists = [data.topByCount, data.topByAvgDuration, data.topBySumDuration, data.topByAvgMemoryUsage];
        for (const list of lists) {
          if (Array.isArray(list)) {
            for (const item of list) {
              queries.add(normalizeQuery(item.query));
            }
          }
        }
        setQuerySet(queries);
      } catch {
        // silently ignore errors - top queries is an optional enhancement
      }
    };

    fetchData();
  }, [fetchUrl]);

  return querySet;
};
