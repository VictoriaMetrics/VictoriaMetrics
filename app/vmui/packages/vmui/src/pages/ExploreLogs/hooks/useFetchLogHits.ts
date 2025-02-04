import { useCallback, useMemo, useRef, useState } from "preact/compat";
import { getLogHitsUrl } from "../../../api/logs";
import { ErrorTypes, TimeParams } from "../../../types";
import { LogHits } from "../../../api/types";
import { useSearchParams } from "react-router-dom";
import { getHitsTimeParams } from "../../../utils/logs";
import { LOGS_GROUP_BY, LOGS_LIMIT_HITS } from "../../../constants/logs";
import { isEmptyObject } from "../../../utils/object";

export const useFetchLogHits = (server: string, query: string) => {
  const [searchParams] = useSearchParams();

  const [logHits, setLogHits] = useState<LogHits[]>([]);
  const [isLoading, setIsLoading] = useState<{[key: number]: boolean;}>([]);
  const [error, setError] = useState<ErrorTypes | string>();
  const abortControllerRef = useRef(new AbortController());

  const url = useMemo(() => getLogHitsUrl(server), [server]);

  const getOptions = (query: string, period: TimeParams, signal: AbortSignal) => {
    const { start, end, step } = getHitsTimeParams(period);

    return {
      signal,
      method: "POST",
      headers: {
        AccountID: searchParams.get("accountID") || "0",
        ProjectID: searchParams.get("projectID") || "0",
      },
      body: new URLSearchParams({
        query: query.trim(),
        step: `${step}ms`,
        start: start.toISOString(),
        end: end.toISOString(),
        fields_limit: `${LOGS_LIMIT_HITS}`,
        field: LOGS_GROUP_BY,
      })
    };
  };

  const fetchLogHits = useCallback(async (period: TimeParams) => {
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;

    const id = Date.now();
    setIsLoading(prev => ({ ...prev, [id]: true }));
    setError(undefined);

    try {
      const options = getOptions(query, period, signal);
      const response = await fetch(url, options);

      if (!response.ok || !response.body) {
        const text = await response.text();
        setError(text);
        setLogHits([]);
        setIsLoading(prev => ({ ...prev, [id]: false }));
        return;
      }

      const data = await response.json();
      const hits = data?.hits as LogHits[];
      if (!hits) {
        const error = "Error: No 'hits' field in response";
        setError(error);
      }

      setLogHits(hits.map(markIsOther).sort(sortHits));
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
        setLogHits([]);
      }
    }
    setIsLoading(prev => ({ ...prev, [id]: false }));
  }, [url, query, searchParams]);

  return {
    logHits,
    isLoading: Object.values(isLoading).some(s => s),
    error,
    fetchLogHits,
    abortController: abortControllerRef.current
  };
};


// Helper function to check if a hit is "other"
const markIsOther = (hit: LogHits) => ({
  ...hit,
  _isOther: isEmptyObject(hit.fields)
});

// Comparison function for sorting hits
const sortHits = (a: LogHits, b: LogHits) => {
  if (a._isOther !== b._isOther) {
    return a._isOther ? -1 : 1; // "Other" hits first to avoid graph overlap
  }
  return b.total - a.total; // Sort remaining by total for better visibility
};
