import {useEffect, useMemo, useCallback, useState} from "preact/compat";
import {getQueryOptions, getQueryRangeUrl, getQueryUrl} from "../../../../api/query-range";
import {useAppState} from "../../../../state/common/StateContext";
import {InstantMetricResult, MetricBase, MetricResult} from "../../../../api/types";
import {isValidHttpUrl} from "../../../../utils/url";
import {useAuthState} from "../../../../state/auth/AuthStateContext";
import {ErrorTypes} from "../../../../types";
import {useGraphState} from "../../../../state/graph/GraphStateContext";
import {getAppModeEnable, getAppModeParams} from "../../../../utils/app-mode";
import throttle from "lodash.throttle";

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

export const useFetchQuery = (): {
  fetchUrl?: string[],
  isLoading: boolean,
  graphData?: MetricResult[],
  liveData?: InstantMetricResult[],
  error?: ErrorTypes | string,
  queryOptions: string[],
} => {
  const {query, displayType, serverUrl, time: {period}, queryControls: {nocache}} = useAppState();

  const {basicData, bearerData, authMethod} = useAuthState();
  const {customStep} = useGraphState();

  const [queryOptions, setQueryOptions] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [graphData, setGraphData] = useState<MetricResult[]>();
  const [liveData, setLiveData] = useState<InstantMetricResult[]>();
  const [error, setError] = useState<ErrorTypes | string>();
  const [fetchQueue, setFetchQueue] = useState<AbortController[]>([]);

  useEffect(() => {
    if (error) {
      setGraphData(undefined);
      setLiveData(undefined);
    }
  }, [error]);

  const fetchData = async (fetchUrl: string[] | undefined) => {
    if (!fetchUrl?.length) return;
    const controller = new AbortController();
    setFetchQueue([...fetchQueue, controller]);
    setIsLoading(true);

    const headers = new Headers();
    if (authMethod === "BASIC_AUTH") {
      headers.set("Authorization", "Basic " + btoa(`${basicData?.login || ""}:${basicData?.password || ""}`));
    }
    if (authMethod === "BEARER_AUTH") {
      headers.set("Authorization", bearerData?.token || "");
    }

    try {
      const responses = await Promise.all(fetchUrl.map(url => fetch(url, {headers, signal: controller.signal})));
      const tempData = [];
      let counter = 1;
      for await (const response of responses) {
        const resp = await response.json();
        if (response.ok) {
          setError(undefined);
          tempData.push(...resp.data.result.map((d: MetricBase) => {
            d.group = counter;
            return d;
          }));
          counter++;
        } else {
          setError(`${resp.errorType}\r\n${resp?.error}`);
        }
      }
      displayType === "chart" ? setGraphData(tempData) : setLiveData(tempData);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }

    setIsLoading(false);
  };

  const throttledFetchData = useCallback(throttle(fetchData, 300), []);

  const fetchOptions = async () => {
    if (!serverUrl) return;
    const url = getQueryOptions(serverUrl);

    try {
      const response = await fetch(url);
      const resp = await response.json();
      if (response.ok) {
        setQueryOptions(resp.data);
      }
    } catch (e) {
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
    }
  };

  const fetchUrl = useMemo(() => {
    const server = appModeEnable ? appServerUrl : serverUrl;
    if (!period) return;
    if (!server) {
      setError(ErrorTypes.emptyServer);
    } else if (query.every(q => !q.trim())) {
      setError(ErrorTypes.validQuery);
    } else if (isValidHttpUrl(server)) {
      if (customStep.enable) period.step = customStep.value;
      return query.filter(q => q.trim()).map(q => displayType === "chart"
        ? getQueryRangeUrl(server, q, period, nocache)
        : getQueryUrl(server, q, period));
    } else {
      setError(ErrorTypes.validServer);
    }
  },
  [serverUrl, period, displayType, customStep]);

  useEffect(() => {
    fetchOptions();
  }, [serverUrl]);

  // TODO: this should depend on query as well, but need to decide when to do the request. Doing it on each query change - looks to be a bad idea. Probably can be done on blur
  useEffect(() => {
    throttledFetchData(fetchUrl);
  }, [fetchUrl]);

  useEffect(() => {
    const fetchPast = fetchQueue.slice(0, -1);
    if (!fetchPast.length) return;
    fetchPast.map(f => f.abort());
    setFetchQueue(fetchQueue.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  return { fetchUrl, isLoading, graphData, liveData, error, queryOptions: queryOptions };
};
