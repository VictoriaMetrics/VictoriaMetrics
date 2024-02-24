import { useCallback, useMemo, useState } from "preact/compat";
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

  const url = useMemo(() => getLogsUrl(server), [server]);

  // include time range in query if not already present
  const queryWithTime = useMemo(() => {
    if (!/_time/.test(query)) {
      const start = dayjs(period.start * 1000).tz().toISOString();
      const end = dayjs(period.end * 1000).tz().toISOString();
      const timerange = `_time:[${start}, ${end}]`;
      return `${timerange} AND ${query}`;
    }
    return query;
  }, [query, period]);

  const options = useMemo(() => ({
    method: "POST",
    headers: {
      "Accept": "application/stream+json",
    },
    body: new URLSearchParams({
      query: queryWithTime.trim(),
      limit: `${limit}`
    })
  }), [queryWithTime, limit]);

  const parseLineToJSON = (line: string): Logs | null => {
    try {
      return JSON.parse(line);
    } catch (e) {
      return null;
    }
  };

  const fetchLogs = useCallback(async () => {
    const limit = Number(options.body.get("limit")) + 1;
    setIsLoading(true);
    setError(undefined);
    try {
      const response = await fetch(url, options);

      if (!response.ok || !response.body) {
        const errorText = await response.text();
        setError(errorText);
        setLogs([]);
        setIsLoading(false);
        return;
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder("utf-8");
      const result = [];

      while (reader) {
        const { done, value } = await reader.read();

        if (done) {
          // "Stream finished, no more data."
          break;
        }

        const lines = decoder.decode(value, { stream: true }).split("\n");
        result.push(...lines);

        // Trim result to limit
        // This will lose its meaning with these changes:
        // https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5778
        if (result.length > limit) {
          result.splice(0, result.length - limit);
        }

        if (result.length >= limit) {
          // Reached the maximum line limit
          reader.cancel();
          break;
        }
      }
      const data = result.map(parseLineToJSON).filter(line => line) as Logs[];
      setLogs(data);
    } catch (e) {
      console.error(e);
      setLogs([]);
      if (e instanceof Error) {
        setError(`${e.name}: ${e.message}`);
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

