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
import usePrevious from "../../hooks/usePrevious";
import { ErrorTypes } from "../../types";
import { useState } from "react";

const ExploreLogs: FC = () => {
  const { serverUrl } = useAppState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [query, setQuery] = useStateSearchParams("", "query");
  const prevQuery = usePrevious(query);
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
    const changedQuery = prevQuery && query !== prevQuery;
    const params: Record<string, string | number> = changedQuery ? { query, page: 1 } : { query };
    setSearchParamsFromKeys(params);
  };

  useEffect(() => {
    if (query) handleRunQuery();
  }, []);

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
