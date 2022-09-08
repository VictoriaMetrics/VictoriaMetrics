import { useEffect, useState } from "react";
import {ErrorTypes} from "../types";
import {getAppModeEnable, getAppModeParams} from "../utils/app-mode";
import {useAppState} from "../state/common/StateContext";
import {useMemo} from "preact/compat";
import {getTopQueries} from "../api/top-queries";
import {TopQueriesData} from "../types";
import {useTopQueriesState} from "../state/topQueries/TopQueriesStateContext";

export const useFetchTopQueries = () => {
  const appModeEnable = getAppModeEnable();
  const {serverURL: appServerUrl} = getAppModeParams();
  const {serverUrl} = useAppState();
  const {topN, maxLifetime, runQuery} = useTopQueriesState();

  const [data, setData] = useState<TopQueriesData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const server = useMemo(() => appModeEnable ? appServerUrl : serverUrl,
    [appModeEnable, serverUrl, appServerUrl]);
  const fetchUrl = useMemo(() => getTopQueries(server, topN, maxLifetime), [server, topN, maxLifetime]);

  const fetchData = async () => {
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();
      setData(response.ok ? resp : null);
      setError(String(resp.error || ""));
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchData();
  }, [runQuery]);

  return {
    data,
    error,
    loading
  };
};
