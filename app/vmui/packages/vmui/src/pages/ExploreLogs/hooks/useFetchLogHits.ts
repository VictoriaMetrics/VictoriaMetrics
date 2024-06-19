import { useCallback, useMemo, useRef, useState } from "preact/compat";
import { getLogHitsUrl } from "../../../api/logs";
import { ErrorTypes } from "../../../types";
import { LogHits } from "../../../api/types";
import { useTimeState } from "../../../state/time/TimeStateContext";
import dayjs from "dayjs";
import { LOGS_BARS_VIEW } from "../../../constants/logs";

export const useFetchLogHits = (server: string, query: string) => {
  const { period } = useTimeState();
  const [logHits, setLogHits] = useState<LogHits[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const abortControllerRef = useRef(new AbortController());

  const url = useMemo(() => getLogHitsUrl(server), [server]);

  const options = useMemo(() => {
    const start = dayjs(period.start * 1000);
    const end = dayjs(period.end * 1000);
    const totalSeconds = end.diff(start, "milliseconds");
    const step = Math.ceil(totalSeconds / LOGS_BARS_VIEW) || 1;

    return {
      method: "POST",
      body: new URLSearchParams({
        query: query.trim(),
        step: `${step}ms`,
        start: start.toISOString(),
        end: end.toISOString(),
      })
    };
  }, [query, period]);

  const fetchLogHits = useCallback(async () => {
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;
    setIsLoading(true);
    setError(undefined);
    try {
      const response = await fetch(url, { ...options, signal });

      if (!response.ok || !response.body) {
        const text = await response.text();
        setError(text);
        setLogHits([]);
        setIsLoading(false);
        return;
      }

      const data = await response.json();
      const hits = data?.hits as LogHits[];
      if (!hits) {
        setError("Error: No 'hits' field in response");
        return;
      }

      setLogHits(hits);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
        setLogHits([]);
      }
    }
    // setIsLoading(false);
  }, [url, options]);

  return {
    logHits,
    isLoading,
    error,
    fetchLogHits,
  };
};
