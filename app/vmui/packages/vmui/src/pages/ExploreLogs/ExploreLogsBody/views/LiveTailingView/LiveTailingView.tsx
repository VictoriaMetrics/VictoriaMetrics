import { FC, useCallback, useEffect, useRef, useState } from "preact/compat";
import { ViewProps } from "../../types";
import useStateSearchParams from "../../../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../../../hooks/useSearchParamsFromObject";
import "./style.scss";
import { useLiveTailingLogs } from "./useLiveTailingLogs";
import { LOGS_DISPLAY_FIELDS, LOGS_URL_PARAMS } from "../../../../../constants/logs";
import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import throttle from "lodash.throttle";
import GroupLogsItem from "../../../GroupLogs/GroupLogsItem";
import LiveTailingSettings from "./LiveTailingSettings";
import Alert from "../../../../../components/Main/Alert/Alert";
import { isDecreasing } from "../../../../../utils/array";
import { useLocalStorageBoolean } from "../../../../../hooks/useLocalStorageBoolean";
import ScrollToTopButton from "../../../../../components/ScrollToTopButton/ScrollToTopButton";

const SCROLL_THRESHOLD = 100;
const scrollToBottom = () => window.scrollTo({
  top: document.documentElement.scrollHeight,
  behavior: "smooth"
});
const throttledScrollToBottom = throttle(scrollToBottom, 200);

const LiveTailingView: FC<ViewProps> = ({ settingsRef }) => {
  const containerRef = useRef<HTMLDivElement>(null);

  const [isAtBottom, setIsAtBottom] = useState(true);
  const [searchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [rowsPerPage, setRowsPerPage] = useStateSearchParams(100, "rows_per_page");
  const [query, _setQuery] = useStateSearchParams("*", "query");
  const [isRawJsonView, setIsRawJsonView] = useLocalStorageBoolean("RAW_JSON_LIVE_VIEW");
  const {
    logs,
    isPaused,
    error,
    startLiveTailing,
    stopLiveTailing,
    pauseLiveTailing,
    resumeLiveTailing,
    clearLogs,
    isLimitedLogsPerUpdate
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

  useEffect(() => {
    startLiveTailing();
    return () => stopLiveTailing();
  }, [startLiveTailing, stopLiveTailing]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let prevScrollTop: number[] = [];
    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = document.documentElement;
      const isBottom = Math.abs(scrollHeight - scrollTop - clientHeight) < SCROLL_THRESHOLD;

      setIsAtBottom(isBottom);
      prevScrollTop.push(scrollTop);
      prevScrollTop = prevScrollTop.slice(-3);
      const isMoveToTop = isDecreasing(prevScrollTop);

      if (!isBottom && !isPaused && isMoveToTop) {
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
        isRawJsonView={isRawJsonView}
        onRawJsonViewChange={setIsRawJsonView}
      />
      <ScrollToTopButton />
      <div
        ref={containerRef}
        className="vm-live-tailing-view__container"
      >
        {logs.length === 0
          ? (<div className="vm-live-tailing-view__empty">Waiting for logs...</div>)
          : (<div className="vm-live-tailing-view__logs">
            {logs.map(({ _log_id, ...log }, idx) =>
              isRawJsonView ? (
                <pre
                  key={idx}
                  className="vm-live-tailing-view__log-row"
                  onMouseDown={pauseLiveTailing}
                >
                  {JSON.stringify(log)}
                </pre>
              ) : (
                <GroupLogsItem
                  key={_log_id}
                  log={log}
                  onItemClick={pauseLiveTailing}
                  hideGroupButton={true}
                  displayFields={displayFields}
                />
              )
            )}
          </div>
          )}
      </div>
      {isLimitedLogsPerUpdate && (
        <Alert variant="warning">Too many logs per second detected. Large volumes of log data are difficult to process
          and may impact performance. We recommend adding filters to your query for better analysis and system
          performance.</Alert>)}
    </>
  );
};

export default LiveTailingView;
