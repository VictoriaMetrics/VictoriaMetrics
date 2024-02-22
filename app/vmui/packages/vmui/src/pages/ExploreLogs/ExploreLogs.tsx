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

const ExploreLogs: FC = () => {
  const { serverUrl } = useAppState();
  const { duration, relativeTime, period } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [query, setQuery] = useStateSearchParams("", "query");
  const { logs, isLoading, error, fetchLogs } = useFetchLogs(serverUrl, query);
  const [queryError, setQueryError] = useState<ErrorTypes | string>("");
  const [loaded, isLoaded] = useState(false);

  const handleRunQuery = () => {
    if (!query) {
      setQueryError(ErrorTypes.validQuery);
      return;
    }

    fetchLogs().then(() => {
      isLoaded(true);
    });

    setSearchParamsFromKeys( {
      query,
      "g0.range_input": duration,
      "g0.end_input": period.date,
      "g0.relative_time": relativeTime || "none",
    });
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
        onChange={setQuery}
        onRun={handleRunQuery}
      />
      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      <ExploreLogsBody
        data={logs}
        loaded={loaded}
      />
    </div>
  );
};

export default ExploreLogs;
