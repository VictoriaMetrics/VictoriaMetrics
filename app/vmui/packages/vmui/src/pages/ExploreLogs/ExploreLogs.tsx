import { FC, useEffect, useMemo, useState } from "preact/compat";
import ExploreLogsBody from "./ExploreLogsBody/ExploreLogsBody";
import useStateSearchParams from "../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../hooks/useSearchParamsFromObject";
import { useFetchLogs } from "./hooks/useFetchLogs";
import { useAppState } from "../../state/common/StateContext";
import Alert from "../../components/Main/Alert/Alert";
import ExploreLogsHeader from "./ExploreLogsHeader/ExploreLogsHeader";
import "./style.scss";
import { ErrorTypes, TimeParams } from "../../types";
import { useTimeState } from "../../state/time/TimeStateContext";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import ExploreLogsBarChart from "./ExploreLogsBarChart/ExploreLogsBarChart";
import { useFetchLogHits } from "./hooks/useFetchLogHits";
import { LOGS_ENTRIES_LIMIT } from "../../constants/logs";
import { getTimeperiodForDuration, relativeTimeOptions } from "../../utils/time";
import { useSearchParams } from "react-router-dom";
import { useQueryDispatch, useQueryState } from "../../state/query/QueryStateContext";
import { getUpdatedHistory } from "../../components/QueryHistory/utils";
import { useDebounceCallback } from "../../hooks/useDebounceCallback";
import usePrevious from "../../hooks/usePrevious";

const storageLimit = Number(getFromStorage("LOGS_LIMIT"));
const defaultLimit = isNaN(storageLimit) ? LOGS_ENTRIES_LIMIT : storageLimit;

type FetchFlags = { logs: boolean; hits: boolean };

const ExploreLogs: FC = () => {
  const { serverUrl } = useAppState();
  const { queryHistory } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const { duration, relativeTime, period: periodState } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [searchParams] = useSearchParams();

  const hideChart = useMemo(() => searchParams.get("hide_chart"), [searchParams]);
  const prevHideChart = usePrevious(hideChart);

  const hideLogs = useMemo(() => searchParams.get("hide_logs"), [searchParams]);
  const prevHideLogs = usePrevious(hideLogs);

  const [limit, setLimit] = useStateSearchParams(defaultLimit, "limit");
  const [query, setQuery] = useStateSearchParams("*", "query");

  const updateHistory = () => {
    const history = getUpdatedHistory(query, queryHistory[0]);
    queryDispatch({
      type: "SET_QUERY_HISTORY",
      payload: {
        key: "LOGS_QUERY_HISTORY",
        history: [history],
      }
    });
  };

  const [isUpdatingQuery, setIsUpdatingQuery] = useState(false);
  const [period, setPeriod] = useState<TimeParams>(periodState);
  const [queryError, setQueryError] = useState<ErrorTypes | string>("");

  const { logs, isLoading, error, fetchLogs, abortController } = useFetchLogs(serverUrl, query, limit);
  const { fetchLogHits, ...dataLogHits } = useFetchLogHits(serverUrl, query);

  const fetchData = async (p: TimeParams, flags: FetchFlags) => {
    if (flags.logs) {
      const isSuccess = await fetchLogs(p);
      if (!isSuccess) return;
    }

    if (flags.hits) {
      await fetchLogHits(p);
    }
  };

  const debouncedFetchLogs = useDebounceCallback(fetchData, 300);

  const getPeriod = () => {
    const relativeTimeOpts = relativeTimeOptions.find(d => d.id === relativeTime);
    if (!relativeTimeOpts) return periodState;
    const { duration, until } = relativeTimeOpts;
    return getTimeperiodForDuration(duration, until());
  };

  const handleRunQuery = () => {
    if (!query) {
      setQueryError(ErrorTypes.validQuery);
      return;
    }
    setQueryError("");

    const newPeriod = getPeriod();
    setPeriod(newPeriod);
    debouncedFetchLogs(newPeriod, { logs: !hideLogs, hits: !hideChart });
    setSearchParamsFromKeys({
      query,
      "g0.range_input": duration,
      "g0.end_input": newPeriod.date,
      "g0.relative_time": relativeTime || "none",
    });
    updateHistory();
  };

  const handleChangeLimit = (limit: number) => {
    setLimit(limit);
    setSearchParamsFromKeys({ limit });
    saveToStorage("LOGS_LIMIT", `${limit}`);
  };

  const handleApplyFilter = (val: string) => {
    setQuery(prev => `${val} AND (${prev})`);
    setIsUpdatingQuery(true);
  };

  const handleUpdateQuery = () => {
    if (isLoading || dataLogHits.isLoading) {
      abortController.abort?.();
      dataLogHits.abortController.abort?.();
    } else {
      handleRunQuery();
    }
  };

  useEffect(() => {
    if (!query) return;
    handleRunQuery();
  }, [periodState]);

  useEffect(() => {
    if (!isUpdatingQuery) return;
    handleRunQuery();
    setIsUpdatingQuery(false);
  }, [query, isUpdatingQuery]);

  useEffect(() => {
    if (!hideChart && prevHideChart) {
      fetchLogHits(period);
    }
  }, [hideChart, prevHideChart, period]);

  useEffect(() => {
    if (!hideLogs && prevHideLogs) {
      fetchLogs(period);
    }
  }, [hideLogs, prevHideLogs, period]);

  return (
    <div className="vm-explore-logs">
      <ExploreLogsHeader
        query={query}
        error={queryError}
        limit={limit}
        onChange={setQuery}
        onChangeLimit={handleChangeLimit}
        onRun={handleUpdateQuery}
        isLoading={isLoading || dataLogHits.isLoading}
      />
      {error && <Alert variant="error">{error}</Alert>}
      {!error && (
        <ExploreLogsBarChart
          {...dataLogHits}
          query={query}
          period={period}
          onApplyFilter={handleApplyFilter}
        />
      )}
      <ExploreLogsBody
        data={logs}
        isLoading={isLoading}
      />
    </div>
  );
};

export default ExploreLogs;
