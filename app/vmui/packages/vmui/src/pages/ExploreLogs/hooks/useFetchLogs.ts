import { useCallback, useMemo, useRef, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes } from "../../../types";
import { Logs } from "../../../api/types";
import { useTimeState } from "../../../state/time/TimeStateContext";
import dayjs from "dayjs";

export const useFetchLogs = (server: string, query: string, limit: number) => {
  const { period } = useTimeState();
  const [logs, setLogs] = useState<Logs[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const abortControllerRef = useRef(new AbortController());

  const url = useMemo(() => getLogsUrl(server), [server]);

  const options = useMemo(() => ({
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
  }), [query, limit, period]);

  const parseLineToJSON = (line: string): Logs | null => {
    try {
      return JSON.parse(line);
    } catch (e) {
      return null;
    }
  };

  const fetchLogs = useCallback(async () => {
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;
    const limit = Number(options.body.get("limit"));
    setIsLoading(true);
    setError(undefined);
    try {
      const response = await fetch(url, { ...options, signal });
      const text = await response.text();

      if (!response.ok || !response.body) {
        setError(text);
        setLogs([]);
        setIsLoading(false);
        return;
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
    }
    setIsLoading(false);
  }, [url, options]);

  return {
    logs,
    isLoading,
    error,
    fetchLogs,
  };
};
