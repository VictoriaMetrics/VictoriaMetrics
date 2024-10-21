import { useCallback, useMemo, useRef, useState } from "preact/compat";
import { getLogsUrl } from "../../../api/logs";
import { ErrorTypes, TimeParams } from "../../../types";
import { Logs } from "../../../api/types";
import dayjs from "dayjs";
import { useSearchParams } from "react-router-dom";

export const useFetchLogs = (server: string, query: string, limit: number) => {
  const [searchParams] = useSearchParams();

  const [logs, setLogs] = useState<Logs[]>([]);
  const [isLoading, setIsLoading] = useState<{[key: number]: boolean;}>([]);
  const [error, setError] = useState<ErrorTypes | string>();
  const abortControllerRef = useRef(new AbortController());

  const url = useMemo(() => getLogsUrl(server), [server]);

  const getOptions = (query: string, period: TimeParams, limit: number, signal: AbortSignal) => ({
    signal,
    method: "POST",
    headers: {
      Accept: "application/stream+json",
      AccountID: searchParams.get("accountID") || "0",
      ProjectID: searchParams.get("projectID") || "0",
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

    const id = Date.now();
    setIsLoading(prev => ({ ...prev, [id]: true }));
    setError(undefined);

    try {
      const options = getOptions(query, period, limit, signal);
      const response = await fetch(url, options);
      const text = await response.text();

      if (!response.ok || !response.body) {
        setError(text);
        setLogs([]);
        setIsLoading(prev => ({ ...prev, [id]: false }));
        return false;
      }

      const lines = text.split("\n").filter(line => line).slice(0, limit);
      const data = lines.map(parseLineToJSON).filter(line => line) as Logs[];
      setLogs(data);
      setIsLoading(prev => ({ ...prev, [id]: false }));
      return true;
    } catch (e) {
      setIsLoading(prev => ({ ...prev, [id]: false }));
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
        setLogs([]);
      }
      return false;
    }
  }, [url, query, limit, searchParams]);

  return {
    logs,
    isLoading: Object.values(isLoading).some(s => s),
    error,
    fetchLogs,
    abortController: abortControllerRef.current
  };
};
