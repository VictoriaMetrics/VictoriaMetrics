import { useCallback, useMemo, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes } from "../../../types";
import { Logs } from "../../../api/types";

const MAX_LINES = 1000;

export const useFetchLogs = (server: string, query: string) => {
  const [logs, setLogs] = useState<Logs[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const url = useMemo(() => getLogsUrl(server), [server]);

  const options = useMemo(() => ({
    method: "POST",
    headers: {
      "Accept": "application/stream+json; charset=utf-8",
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: `query=${encodeURIComponent(query.trim())}`
  }), [query]);

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

