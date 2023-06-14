import { useCallback, useMemo, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes } from "../../../types";
import { Logs } from "../../../api/types";

export const useFetchLogs = (server: string, query: string) => {
  const [logs, setLogs] = useState<Logs[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const url = useMemo(() => getLogsUrl(server), [server]);

  const options = useMemo(() => ({
    method: "POST",
    headers: {
      // TODO replace to json
      "Content-Type": "application/x-www-form-urlencoded"
    },
    body: `query=${encodeURIComponent(query)}`
  }), [query]);

  const fetchLogs = useCallback(async () => {
    setIsLoading(true);
    try {
      const response = await fetch(url, options);
      // TODO replace to json()
      // const resp = await response.json();
      const resp = await response.text();
      const arr = resp.split("\n");
      const data = arr.filter(el => el).map(el => JSON.parse(el));
      setLogs(data);
      if (response.ok) {
        setError(undefined);
      } else {
        // TODO uncomment when replace to json()
        // setError(`${resp.errorType}\r\n${resp?.error}`);
      }
    } catch (e) {
      console.error(e);
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

