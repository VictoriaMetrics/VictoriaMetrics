import { useCallback, useMemo, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes } from "../../../types";
import { Logs } from "../../../api/types";
import { useTimeState } from "../../../state/time/TimeStateContext";
import dayjs from "dayjs";

const MAX_LINES = 1000;

export const useFetchLogs = (server: string, query: string) => {
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
      "Accept": "application/stream+json; charset=utf-8",
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: `query=${encodeURIComponent(queryWithTime.trim())}`
  }), [queryWithTime]);

  const fetchLogs = useCallback(async () => {
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

        // Trim result to MAX_LINES
        if (result.length > MAX_LINES) {
          result.splice(0, result.length - MAX_LINES);
        }

        if (result.length >= MAX_LINES) {
          // Reached the maximum line limit
          reader.cancel();
          break;
        }
      }
      const data = result
        .map((line) => {
          try {
            return JSON.parse(line);
          } catch (e) {
            return "";
          }
        })
        .filter(line => line);

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

