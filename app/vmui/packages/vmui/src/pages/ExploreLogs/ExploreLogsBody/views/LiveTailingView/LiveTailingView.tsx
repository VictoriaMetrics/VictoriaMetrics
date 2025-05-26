import { FC, useCallback, useEffect, useRef, useState } from "preact/compat";
import { ViewProps } from "../../types";
import useStateSearchParams from "../../../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../../../hooks/useSearchParamsFromObject";
import "./style.scss";
import { useLiveTailingLogs } from "./useLiveTailingLogs";
import { LOGS_DISPLAY_FIELDS, LOGS_URL_PARAMS } from "../../../../../constants/logs";
import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import throttle from "lodash/throttle";
import GroupLogsItem from "../../../GroupLogs/GroupLogsItem";
import LiveTailingSettings from "./LiveTailingSettings";

const SCROLL_THRESHOLD = 100;
const scrollToBottom = () => window.scrollTo({
  top: document.documentElement.scrollHeight,
  behavior: "instant"
});
const throttledScrollToBottom = throttle(scrollToBottom, 200);

const LiveTailingView: FC<ViewProps> = ({ settingsRef }) => {
  const containerRef = useRef<HTMLDivElement>(null);

  const [isAtBottom, setIsAtBottom] = useState(true);
  const [searchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [rowsPerPage, setRowsPerPage] = useStateSearchParams(100, "rows_per_page");
  const [query, _setQuery] = useStateSearchParams("*", "query");
  const [isCompactTailingStr] = useStateSearchParams(0, "compact_tailing");
  const isCompactTailingNumber = Boolean(Number(isCompactTailingStr));
  const {
    logs,
    isPaused,
    error,
    startLiveTailing,
    stopLiveTailing,
    pauseLiveTailing,
    resumeLiveTailing,
    clearLogs
  } = useLiveTailingLogs(query, rowsPerPage);

  const displayFieldsString = searchParams.get(LOGS_URL_PARAMS.DISPLAY_FIELDS) || LOGS_DISPLAY_FIELDS;
  const displayFields = useMemo(() => displayFieldsString.split(","), [displayFieldsString]);

  const handleResumeLiveTailing = useCallback(() => {
    throttledScrollToBottom();
    resumeLiveTailing();
  }, [resumeLiveTailing]);

  const handleSetRowsPerPage = useCallback((limit: number) => {
    setSearchParamsFromKeys({ rows_per_page: limit });
  }, [setRowsPerPage, setSearchParamsFromKeys]);

  const handleSetCompactTailing = useCallback((value: boolean) => {
    setSearchParamsFromKeys({ compact_tailing: Number(value) });
  }, [setSearchParamsFromKeys]);

  useEffect(() => {
    startLiveTailing();
    return () => stopLiveTailing();
  }, [startLiveTailing, stopLiveTailing]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = document.documentElement;
      const isBottom = Math.abs(scrollHeight - scrollTop - clientHeight) < SCROLL_THRESHOLD;

      setIsAtBottom(isBottom);

      if (!isBottom && !isPaused) {
        pauseLiveTailing();
      }
    };

    document.addEventListener("scroll", handleScroll);
    return () => document.removeEventListener("scroll", handleScroll);
  }, [isPaused, pauseLiveTailing, resumeLiveTailing]);

  useEffect(() => {
    if (isAtBottom && !isPaused) {
      throttledScrollToBottom();
    }
  }, [logs, isAtBottom]);

  useEffect(() => {
    handleResumeLiveTailing();
  }, [rowsPerPage]);



  if (error) {
    return <div className="vm-live-tailing-view__error">{error}</div>;
  }

  return (
    <>
      <LiveTailingSettings
        settingsRef={settingsRef}
        rowsPerPage={rowsPerPage}
        handleSetRowsPerPage={handleSetRowsPerPage}
        logs={logs}
        isPaused={isPaused}
        handleResumeLiveTailing={handleResumeLiveTailing}
        pauseLiveTailing={pauseLiveTailing}
        clearLogs={clearLogs}
        isCompactTailingNumber={isCompactTailingNumber}
        handleSetCompactTailing={handleSetCompactTailing}
      />
      <div
        ref={containerRef}
        className="vm-live-tailing-view__container"
      >
        {logs.length === 0
          ? (<div className="vm-live-tailing-view__empty">Waiting for logs...</div>)
          : (<div className="vm-live-tailing-view__logs">
            {logs.map(({ _log_id, ...log }, idx) =>
              isCompactTailingNumber
                ? (
                  <GroupLogsItem
                    key={_log_id}
                    log={log}
                    onItemClick={pauseLiveTailing}
                    hideGroupButton={true}
                    displayFields={displayFields}
                  />
                ) : (
                  <pre
                    key={idx}
                    className="vm-live-tailing-view__log-row"
                  >
                    {JSON.stringify(log)}
                  </pre>
                )
            )}
          </div>
          )}
      </div>
    </>
  );
};

export default LiveTailingView;
