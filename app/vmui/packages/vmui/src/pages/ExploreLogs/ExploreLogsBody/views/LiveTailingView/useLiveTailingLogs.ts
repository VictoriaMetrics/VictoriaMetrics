import { useCallback, useEffect, useRef, useState } from "preact/compat";
import { ErrorTypes } from "../../../../../types";
import { Logs } from "../../../../../api/types";
import { useAppState } from "../../../../../state/common/StateContext";
import { useSearchParams } from "react-router-dom";
import useBoolean from "../../../../../hooks/useBoolean";

export const useLiveTailingLogs = (query: string, limit: number) => {
  const { serverUrl } = useAppState();
  const [searchParams] = useSearchParams();

  const [logs, setLogs] = useState<Logs[]>([]);
  const { value: isPaused, setTrue: pauseLiveTailing, setFalse: resumeLiveTailing } = useBoolean(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const counterRef = useRef<bigint>(0n);
  const abortControllerRef = useRef(new AbortController());
  const readerRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const bufferRef = useRef<string>("");

  const stopLiveTailing = useCallback(() => {
    if (readerRef.current) {
      readerRef.current.cancel();
      readerRef.current = null;
    }
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
    }
    if (bufferRef.current) {
      bufferRef.current = "";
    }
    abortControllerRef.current.abort();
  }, []);

  const startLiveTailing = useCallback(async () => {
    stopLiveTailing();

    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;

    setError(undefined);
    setLogs([]);

    try {
      const tenant = {
        AccountID: searchParams.get("accountID") || "0",
        ProjectID: searchParams.get("projectID") || "0"
      };
      const response = await fetch(`${serverUrl}/select/logsql/tail`, {
        signal,
        method: "POST",
        headers: {
          ...tenant,
        },
        body: new URLSearchParams({
          query: query.trim(),
        })
      });

      if (!response.ok || !response.body) {
        const text = await response.text();
        setError(text);
        setLogs([]);
        return false;
      }

      const reader = response.body.getReader();
      readerRef.current = reader;

      const processStream = async () => {
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            // Convert the Uint8Array to a string
            const chunk = new TextDecoder().decode(value);
            bufferRef.current += chunk;
          }
        } catch (e) {
          if (e instanceof Error && e.name !== "AbortError") {
            console.error("Stream processing error:", e);
            setError(String(e));
          }
        }
      };

      processStream();
      return true;
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
        setLogs([]);
      }
      return false;
    }
  }, [query, stopLiveTailing]);


  useEffect(() => {
    if (isPaused) return;

    /**
     * Process incoming log data at a throttled rate (every 1s)
     * This interval-based approach prevents CPU overload by:
     * 1. Batching log processing instead of processing each chunk immediately
     * 2. Limiting UI updates to a reasonable frequency (1/sec) even when data streams in rapidly
     * 3. Reducing performance impact when handling large volumes of incoming logs
     * 4. Allowing efficient garbage collection between processing cycles
     */
    const timerId = setInterval(() => {
      const lines = bufferRef.current.split("\n");
      bufferRef.current = lines.pop() || "";

      const newLogs = lines
        .map(line => {
          try {
            const parsedLine = line && JSON.parse(line);
            parsedLine._log_id = counterRef.current++;
            return parsedLine;
          } catch (e) {
            console.error(`Failed to parse "${line}" to JSON\n`, e);
            return null;
          }
        })
        .filter(Boolean) as Logs[];

      setLogs(prevLogs => {
        const combinedLogs = [...prevLogs, ...newLogs];
        return combinedLogs.length > limit ? combinedLogs.slice(-limit) : combinedLogs;
      });
    }, 1000);
    return () => clearInterval(timerId);
  }, [limit, isPaused]);

  const clearLogs = useCallback(() => {
    setLogs([]);
  }, []);

  return {
    logs,
    isPaused,
    error,
    startLiveTailing,
    stopLiveTailing,
    pauseLiveTailing,
    resumeLiveTailing,
    clearLogs
  };
}; 