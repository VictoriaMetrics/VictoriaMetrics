import { useCallback, useEffect, useRef, useState } from "preact/compat";
import { ErrorTypes } from "../../../../../types";
import { Logs } from "../../../../../api/types";
import { useAppState } from "../../../../../state/common/StateContext";
import useBoolean from "../../../../../hooks/useBoolean";
import { useTenant } from "../../../../../hooks/useTenant";

/**
 * Defines the maximum number of consecutive times logs can be fetched above the threshold
 * before showing a warning notification, and vice versa:
 * - If logs are fetched above a threshold this many times in a row -> show warning
 * - If warning is shown, it won't disappear until logs are fetched below a threshold
 *   this many times in a row
 *
 * This threshold helps optimize log display performance when dealing with large volumes of logs.
 * If the threshold is consistently exceeded, users will be prompted to add filters to their query
 * for better system performance and more focused log analysis.
 */
const MAX_ATTEMPTS_FETCH_LOGS_PER_SECOND = 5;
/**
 * Defines the log's threshold, after which will be shown a warning notification
 */
const LOGS_THRESHOLD = 200;
const CONNECTION_TIMEOUT_MS = 5000;
const PROCESSING_INTERVAL_MS = 1000;

const createStreamProcessor = (
  bufferRef: React.MutableRefObject<string>,
  bufferLinesRef: React.MutableRefObject<string[]>,
  setError: (error: string) => void,
  restartTailing: () => Promise<boolean>
) => {
  return async (reader: ReadableStreamDefaultReader<Uint8Array>) => {
    let lastDataTime = Date.now();

    const connectionCheckInterval = setInterval(() => {
      const timeSinceLastData = Date.now() - lastDataTime;
      if (timeSinceLastData > CONNECTION_TIMEOUT_MS) {
        clearInterval(connectionCheckInterval);
        restartTailing();
        return;
      }
    }, CONNECTION_TIMEOUT_MS);

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        lastDataTime = Date.now();

        const chunk = new TextDecoder().decode(value);
        const lines = (bufferRef.current + chunk).split("\n");
        bufferRef.current = lines.pop() || "";
        bufferLinesRef.current = [...bufferLinesRef.current, ...lines];
      }
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        console.error("Stream processing error:", e);
        setError(String(e));
      }
    } finally {
      clearInterval(connectionCheckInterval);
    }
  };
};

const updateLimitModeTracking = (
  linesCount: number,
  attemptsFetchLimitRef: React.MutableRefObject<number>,
  attemptsFetchLowRef: React.MutableRefObject<number>,
  isLimitedLogsPerUpdate: boolean,
) => {
  if (linesCount > LOGS_THRESHOLD) {
    attemptsFetchLimitRef.current++;
    attemptsFetchLowRef.current = 0;
  } else {
    attemptsFetchLowRef.current++;
    attemptsFetchLimitRef.current = 0;
  }

  if (attemptsFetchLimitRef.current > MAX_ATTEMPTS_FETCH_LOGS_PER_SECOND) {
    return true;
  }

  if (attemptsFetchLowRef.current > MAX_ATTEMPTS_FETCH_LOGS_PER_SECOND) {
    return false;
  }

  return isLimitedLogsPerUpdate;
};

const parseLogLines = (lines: string[], counterRef: React.MutableRefObject<bigint>): Logs[] => {
  return lines
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
};

interface ProcessBufferedLogsParams {
  lines: string[];
  limit: number;
  counterRef: React.MutableRefObject<bigint>;
  attemptsFetchLimitRef: React.MutableRefObject<number>;
  attemptsFetchLowRef: React.MutableRefObject<number>;
  setIsLimitedLogsPerUpdate: (isLimited: boolean) => void;
  setLogs: React.Dispatch<React.SetStateAction<Logs[]>>;
  bufferLinesRef: React.MutableRefObject<string[]>;
  isLimitedLogsPerUpdate: boolean;
}

const processBufferedLogs = ({
  lines,
  limit,
  counterRef,
  attemptsFetchLimitRef,
  attemptsFetchLowRef,
  setIsLimitedLogsPerUpdate,
  setLogs,
  bufferLinesRef,
  isLimitedLogsPerUpdate
}: ProcessBufferedLogsParams) => {

  const isLimitLogsMode = updateLimitModeTracking(lines.length, attemptsFetchLimitRef, attemptsFetchLowRef, isLimitedLogsPerUpdate);
  const limitedLines = isLimitLogsMode && lines.length > LOGS_THRESHOLD ? lines.slice(-LOGS_THRESHOLD) : lines;
  const newLogs = parseLogLines(limitedLines, counterRef);

  setIsLimitedLogsPerUpdate(isLimitLogsMode);
  setLogs(prevLogs => {
    const combinedLogs = [...prevLogs, ...newLogs];
    return combinedLogs.length > limit ? combinedLogs.slice(-limit) : combinedLogs;
  });
  bufferLinesRef.current = [];
};

export const useLiveTailingLogs = (query: string, limit: number) => {
  const { serverUrl } = useAppState();

  const [logs, setLogs] = useState<Logs[]>([]);
  const { value: isPaused, setTrue: pauseLiveTailing, setFalse: resumeLiveTailing } = useBoolean(false);
  const tenant = useTenant();
  const [error, setError] = useState<ErrorTypes | string>();
  const [isLimitedLogsPerUpdate, setIsLimitedLogsPerUpdate] = useState(false);

  const counterRef = useRef<bigint>(0n);
  const abortControllerRef = useRef(new AbortController());
  const readerRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const bufferRef = useRef<string>("");
  const bufferLinesRef = useRef<string[]>([]);
  const attemptsFetchLimitLogsPerSecondCountRef = useRef<number>(0);
  const attemptsFetchLowLogsPerSecondCountRef = useRef<number>(0);

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

    try {
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

      const processStream = createStreamProcessor(
        bufferRef,
        bufferLinesRef,
        setError,
        startLiveTailing
      );

      processStream(reader);
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
    if (isPaused) {
      const pauseTimerId = setInterval(() => {
        if (bufferLinesRef.current.length > limit) {
          bufferLinesRef.current = bufferLinesRef.current.slice(-limit);
        }
      }, PROCESSING_INTERVAL_MS);
      return () => {
        clearInterval(pauseTimerId);
      };
    }

    const timerId = setInterval(() => {
      const lines = bufferLinesRef.current;
      processBufferedLogs({
        lines,
        limit,
        counterRef,
        attemptsFetchLimitRef: attemptsFetchLimitLogsPerSecondCountRef,
        attemptsFetchLowRef: attemptsFetchLowLogsPerSecondCountRef,
        setIsLimitedLogsPerUpdate,
        isLimitedLogsPerUpdate,
        setLogs,
        bufferLinesRef
      });
    }, PROCESSING_INTERVAL_MS);

    return () => clearInterval(timerId);
  }, [limit, isPaused, isLimitedLogsPerUpdate]);

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
    clearLogs,
    isLimitedLogsPerUpdate
  };
}; 
