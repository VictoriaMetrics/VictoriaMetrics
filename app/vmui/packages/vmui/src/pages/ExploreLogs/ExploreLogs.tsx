import React, { FC, useEffect } from "preact/compat";
import ExploreLogsBody from "./ExploreLogsBody/ExploreLogsBody";
import useStateSearchParams from "../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../hooks/useSearchParamsFromObject";
import { useFetchLogs } from "./hooks/useFetchLogs";
import { useAppState } from "../../state/common/StateContext";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import ExploreLogsHeader from "./ExploreLogsHeader/ExploreLogsHeader";
import "./style.scss";
import { ErrorTypes } from "../../types";
import { useState } from "react";
import { useTimeState } from "../../state/time/TimeStateContext";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import ExploreLogsBarChart from "./ExploreLogsBarChart/ExploreLogsBarChart";
import { useFetchLogHits } from "./hooks/useFetchLogHits";
import { LOGS_ENTRIES_LIMIT } from "../../constants/logs";

const storageLimit = Number(getFromStorage("LOGS_LIMIT"));
const defaultLimit = isNaN(storageLimit) ? LOGS_ENTRIES_LIMIT : storageLimit;

const ExploreLogs: FC = () => {
  const { serverUrl } = useAppState();
  const { duration, relativeTime, period } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [limit, setLimit] = useStateSearchParams(defaultLimit, "limit");
  const [query, setQuery] = useStateSearchParams("*", "query");
  const { logs, isLoading, error, fetchLogs } = useFetchLogs(serverUrl, query, limit);
  const { fetchLogHits, ...dataLogHits } = useFetchLogHits(serverUrl, query);
  const [queryError, setQueryError] = useState<ErrorTypes | string>("");
  const [loaded, isLoaded] = useState(false);
  const [markdownParsing, setMarkdownParsing] = useState(getFromStorage("LOGS_MARKDOWN") === "true");

  const handleRunQuery = () => {
    if (!query) {
      setQueryError(ErrorTypes.validQuery);
      return;
    }

    fetchLogs().then(() => {
      isLoaded(true);
    });
    fetchLogHits();

    setSearchParamsFromKeys( {
      query,
      "g0.range_input": duration,
      "g0.end_input": period.date,
      "g0.relative_time": relativeTime || "none",
    });
  };

  const handleChangeLimit = (limit: number) => {
    setLimit(limit);
    setSearchParamsFromKeys({ limit });
    saveToStorage("LOGS_LIMIT", `${limit}`);
  };

  const handleChangeMarkdownParsing = (val: boolean) => {
    saveToStorage("LOGS_MARKDOWN", `${val}`);
    setMarkdownParsing(val);
  };

  useEffect(() => {
    if (query) handleRunQuery();
  }, [period]);

  useEffect(() => {
    setQueryError("");
  }, [query]);

  return (
    <div className="vm-explore-logs">
      <ExploreLogsHeader
        query={query}
        error={queryError}
        limit={limit}
        markdownParsing={markdownParsing}
        onChange={setQuery}
        onChangeLimit={handleChangeLimit}
        onRun={handleRunQuery}
        onChangeMarkdownParsing={handleChangeMarkdownParsing}
      />
      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      <ExploreLogsBarChart
        query={query}
        loaded={loaded}
        {...dataLogHits}
      />
      <ExploreLogsBody
        data={logs}
        loaded={loaded}
        markdownParsing={markdownParsing}
      />
    </div>
  );
};

export default ExploreLogs;
