import {useCallback, useEffect, useMemo, useState} from "preact/compat";
import {getQueryRangeUrl, getQueryUrl} from "../api/query-range";
import {useAppState} from "../state/common/StateContext";
import {InstantMetricResult, MetricBase, MetricResult} from "../api/types";
import {isValidHttpUrl} from "../utils/url";
import {ErrorTypes} from "../types";
import {getAppModeEnable, getAppModeParams} from "../utils/app-mode";
import debounce from "lodash.debounce";
import {DisplayType} from "../components/CustomPanel/Configurator/DisplayTypeSwitch";
import usePrevious from "./usePrevious";
import {arrayEquals} from "../utils/array";
import Trace from "../components/CustomPanel/Trace/Trace";
import {MAX_SERIES} from "../config";

interface FetchQueryParams {
  predefinedQuery?: string[]
  visible: boolean
  display?: DisplayType,
  customStep: number,
}

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

export const useFetchQuery = ({predefinedQuery, visible, display, customStep}: FetchQueryParams): {
  fetchUrl?: string[],
  isLoading: boolean,
  graphData?: MetricResult[],
  liveData?: InstantMetricResult[],
  error?: ErrorTypes | string,
  warning?: string,
  traces?: Trace[],
} => {
  const {query, displayType, serverUrl, time: {period}, queryControls: {nocache, isTracingEnabled}} = useAppState();

  const [isLoading, setIsLoading] = useState(false);
  const [graphData, setGraphData] = useState<MetricResult[]>();
  const [liveData, setLiveData] = useState<InstantMetricResult[]>();
  const [traces, setTraces] = useState<Trace[]>();
  const [error, setError] = useState<ErrorTypes | string>();
  const [warning, setWarning] = useState<string>();
  const [fetchQueue, setFetchQueue] = useState<AbortController[]>([]);

  useEffect(() => {
    if (error) {
      setGraphData(undefined);
      setLiveData(undefined);
      setTraces(undefined);
    }
  }, [error]);

  const fetchData = async (fetchUrl: string[], fetchQueue: AbortController[], displayType: DisplayType, query: string[]) => {
    const controller = new AbortController();
    setFetchQueue([...fetchQueue, controller]);
    const isDisplayChart = displayType === "chart";
    try {
      const responses = await Promise.all(fetchUrl.map(url => fetch(url, {signal: controller.signal})));
      const tempData: MetricBase[] = [];
      const tempTraces: Trace[] = [];
      let counter = 1;

      for await (const response of responses) {
        const resp = await response.json();
        if (response.ok) {
          setError(undefined);
          if (resp.trace) {
            const trace = new Trace(resp.trace, query[counter-1]);
            tempTraces.push(trace);
          }
          resp.data.result.forEach((d: MetricBase) => {
            d.group = counter;
            tempData.push(d);
          });
          counter++;
        } else {
          setError(`${resp.errorType}\r\n${resp?.error}`);
        }
      }

      const length = tempData.length;
      const seriesLimit = MAX_SERIES[displayType];
      const result = tempData.slice(0, seriesLimit);
      const limitText = `Showing ${seriesLimit} series out of ${length} series due to performance reasons. Please narrow down the query, so it returns less series`;
      setWarning(length > seriesLimit ? limitText : "");

      isDisplayChart ? setGraphData(result as MetricResult[]) : setLiveData(result as InstantMetricResult[]);
      setTraces(tempTraces);
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
      updatedPeriod.step = customStep;
      return expr.filter(q => q.trim()).map(q => displayChart
        ? getQueryRangeUrl(server, q, updatedPeriod, nocache, isTracingEnabled)
        : getQueryUrl(server, q, updatedPeriod, isTracingEnabled));
    } else {
      setError(ErrorTypes.validServer);
    }
  },
  [serverUrl, period, displayType, customStep]);

  const prevFetchUrl = usePrevious(fetchUrl);
  const prevDisplayType = usePrevious(displayType);

  useEffect(() => {
    const equalFetchUrl = fetchUrl && prevFetchUrl && arrayEquals(fetchUrl, prevFetchUrl);
    const changedDisplayType = displayType !== prevDisplayType;
    if (!visible || (equalFetchUrl && !changedDisplayType) || !fetchUrl?.length) return;
    setIsLoading(true);
    const expr = predefinedQuery ?? query;
    throttledFetchData(fetchUrl, fetchQueue, (display || displayType), expr);
  }, [fetchUrl, visible]);

  useEffect(() => {
    const fetchPast = fetchQueue.slice(0, -1);
    if (!fetchPast.length) return;
    fetchPast.map(f => f.abort());
    setFetchQueue(fetchQueue.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  return {fetchUrl, isLoading, graphData, liveData, error, warning, traces};
};
