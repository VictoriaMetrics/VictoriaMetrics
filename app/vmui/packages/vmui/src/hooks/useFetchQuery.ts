import {useEffect, useMemo, useCallback, useState} from "preact/compat";
import {getQueryRangeUrl, getQueryUrl} from "../api/query-range";
import {useAppState} from "../state/common/StateContext";
import {InstantMetricResult, MetricBase, MetricResult} from "../api/types";
import {isValidHttpUrl} from "../utils/url";
import {ErrorTypes} from "../types";
import {getAppModeEnable, getAppModeParams} from "../utils/app-mode";
import debounce from "lodash.debounce";
import {DisplayType} from "../components/CustomPanel/Configurator/DisplayTypeSwitch";
import {CustomStep} from "../state/graph/reducer";
import usePrevious from "./usePrevious";
import {arrayEquals} from "../utils/array";

interface FetchQueryParams {
  predefinedQuery?: string[]
  visible: boolean
  display?: DisplayType,
  customStep: CustomStep,
}

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

export const useFetchQuery = ({predefinedQuery, visible, display, customStep}: FetchQueryParams): {
  fetchUrl?: string[],
  isLoading: boolean,
  graphData?: MetricResult[],
  liveData?: InstantMetricResult[],
  error?: ErrorTypes | string,
} => {
  const {query, displayType, serverUrl, time: {period}, queryControls: {nocache}} = useAppState();

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

  const fetchData = async (fetchUrl: string[], fetchQueue: AbortController[], displayType: DisplayType) => {
    const controller = new AbortController();
    setFetchQueue([...fetchQueue, controller]);
    try {
      const responses = await Promise.all(fetchUrl.map(url => fetch(url, {signal: controller.signal})));
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

  const throttledFetchData = useCallback(debounce(fetchData, 600), []);

  const fetchUrl = useMemo(() => {
    const server = appModeEnable ? appServerUrl : serverUrl;
    const expr = predefinedQuery ?? query;
    const displayChart = (display || displayType) === "chart";
    if (!period) return;
    if (!server) {
      setError(ErrorTypes.emptyServer);
    } else if (expr.every(q => !q.trim())) {
      setError(ErrorTypes.validQuery);
    } else if (isValidHttpUrl(server)) {
      const updatedPeriod = {...period};
      if (customStep.enable) updatedPeriod.step = customStep.value;
      return expr.filter(q => q.trim()).map(q => displayChart
        ? getQueryRangeUrl(server, q, updatedPeriod, nocache)
        : getQueryUrl(server, q, updatedPeriod));
    } else {
      setError(ErrorTypes.validServer);
    }
  },
  [serverUrl, period, displayType, customStep]);

  const prevFetchUrl = usePrevious(fetchUrl);

  useEffect(() => {
    if (!visible || (fetchUrl && prevFetchUrl && arrayEquals(fetchUrl, prevFetchUrl)) || !fetchUrl?.length) return;
    setIsLoading(true);
    throttledFetchData(fetchUrl, fetchQueue, (display || displayType));
  }, [fetchUrl, visible]);

  useEffect(() => {
    const fetchPast = fetchQueue.slice(0, -1);
    if (!fetchPast.length) return;
    fetchPast.map(f => f.abort());
    setFetchQueue(fetchQueue.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  return { fetchUrl, isLoading, graphData, liveData, error };
};
