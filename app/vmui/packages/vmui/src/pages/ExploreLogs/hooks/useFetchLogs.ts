import { useCallback, useMemo, useRef, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes, TimeParams } from "../../../types";
import { Logs } from "../../../api/types";
import dayjs from "dayjs";

export const useFetchLogs = (server: string, query: string, limit: number) => {
  const [logs, setLogs] = useState<Logs[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const abortControllerRef = useRef(new AbortController());

  const url = useMemo(() => getLogsUrl(server), [server]);

  const getOptions = (query: string, period: TimeParams, limit: number, signal: AbortSignal) => ({
    signal,
    method: "POST",
    headers: {
      "Accept": "application/stream+json",
    },
    body: new URLSearchParams({
      query: query.trim(),
      limit: `${limit}`,
      start: dayjs(period.start * 1000).tz().toISOString(),
      end: dayjs(period.end * 1000).tz().toISOString()
    })
  });

  const parseLineToJSON = (line: string): Logs | null => {
    try {
      return JSON.parse(line);
    } catch (e) {
      return null;
    }
  };

  const fetchLogs = useCallback(async (period: TimeParams) => {
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;

    setIsLoading(true);
    setError(undefined);

    try {
      const options = getOptions(query, period, limit, signal);
      const response = await fetch(url, options);
      const text = await response.text();

      if (!response.ok || !response.body) {
        setError(text);
        setLogs([]);
        setIsLoading(false);
        return Promise.reject(new Error(text));
      }

      const lines = text.split("\n").filter(line => line).slice(0, limit);
      const data = lines.map(parseLineToJSON).filter(line => line) as Logs[];
      setLogs(data);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
        setLogs([]);
      }
      return Promise.reject(e);
    } finally {
      setIsLoading(false);
    }
    setIsLoading(false);
  }, [url, query, limit]);

  return {
    logs,
    isLoading,
    error,
    fetchLogs,
  };
};
