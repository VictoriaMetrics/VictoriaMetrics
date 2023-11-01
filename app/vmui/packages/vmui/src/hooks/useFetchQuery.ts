import { useCallback, useEffect, useMemo, useState } from "preact/compat";
import { StateUpdater } from "preact/hooks";
import { getQueryRangeUrl, getQueryUrl } from "../api/query-range";
import { useAppState } from "../state/common/StateContext";
import { InstantMetricResult, MetricBase, MetricResult, QueryStats } from "../api/types";
import { isValidHttpUrl } from "../utils/url";
import { ErrorTypes, SeriesLimits } from "../types";
import debounce from "lodash.debounce";
import { DisplayType } from "../pages/CustomPanel/DisplayTypeSwitch";
import Trace from "../components/TraceQuery/Trace";
import { useQueryState } from "../state/query/QueryStateContext";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCustomPanelState } from "../state/customPanel/CustomPanelStateContext";
import { isHistogramData } from "../utils/metric";
import { useGraphState } from "../state/graph/GraphStateContext";
import { getStepFromDuration } from "../utils/time";

interface FetchQueryParams {
  predefinedQuery?: string[]
  visible: boolean
  display?: DisplayType,
  customStep: string,
  hideQuery?: number[]
  showAllSeries?: boolean
}

interface FetchQueryReturn {
  fetchUrl?: string[],
  isLoading: boolean,
  graphData?: MetricResult[],
  liveData?: InstantMetricResult[],
  error?: ErrorTypes | string,
  queryErrors: (ErrorTypes | string)[],
  setQueryErrors: StateUpdater<string[]>,
  queryStats: QueryStats[],
  warning?: string,
  traces?: Trace[],
  isHistogram: boolean
}

interface FetchDataParams {
  fetchUrl: string[],
  fetchQueue: AbortController[],
  displayType: DisplayType,
  query: string[],
  stateSeriesLimits: SeriesLimits,
  showAllSeries?: boolean,
  hideQuery?: number[]
}

export const useFetchQuery = ({
  predefinedQuery,
  visible,
  display,
  customStep,
  hideQuery,
  showAllSeries
}: FetchQueryParams): FetchQueryReturn => {
  const { query } = useQueryState();
  const { period } = useTimeState();
  const { displayType, nocache, isTracingEnabled, seriesLimits: stateSeriesLimits } = useCustomPanelState();
  const { serverUrl } = useAppState();
  const { isHistogram: isHistogramState } = useGraphState();

  const [isLoading, setIsLoading] = useState(false);
  const [graphData, setGraphData] = useState<MetricResult[]>();
  const [liveData, setLiveData] = useState<InstantMetricResult[]>();
  const [traces, setTraces] = useState<Trace[]>();
  const [error, setError] = useState<ErrorTypes | string>();
  const [queryErrors, setQueryErrors] = useState<string[]>([]);
  const [queryStats, setQueryStats] = useState<QueryStats[]>([]);
  const [warning, setWarning] = useState<string>();
  const [fetchQueue, setFetchQueue] = useState<AbortController[]>([]);
  const [isHistogram, setIsHistogram] = useState(false);

  const defaultStep = useMemo(() => {
    const { end, start } = period;
    return getStepFromDuration(end - start, isHistogramState);
  }, [period, isHistogramState]);

  const fetchData = async ({
    fetchUrl,
    fetchQueue,
    displayType,
    query,
    stateSeriesLimits,
    showAllSeries,
    hideQuery,
  }: FetchDataParams) => {
    const controller = new AbortController();
    setFetchQueue([...fetchQueue, controller]);
    try {
      const isDisplayChart = displayType === "chart";
      const defaultLimit = showAllSeries ? Infinity : (+stateSeriesLimits[displayType] || Infinity);
      let seriesLimit = defaultLimit;
      const tempData: MetricBase[] = [];
      const tempTraces: Trace[] = [];
      let counter = 1;
      let totalLength = 0;
      let isHistogramResult = false;

      for await (const url of fetchUrl) {

        const isHideQuery = hideQuery?.includes(counter - 1);
        if (isHideQuery) {
          setQueryErrors(prev => [...prev, ""]);
          setQueryStats(prev => [...prev, {}]);
          counter++;
          continue;
        }

        const response = await fetch(url, { signal: controller.signal });
        const resp = await response.json();

        if (response.ok) {
          setQueryStats(prev => [...prev, {
            ...resp?.stats,
            isPartial: resp?.isPartial,
            resultLength: resp.data.result.length,
          }]);
          setQueryErrors(prev => [...prev, ""]);

          if (resp.trace) {
            const trace = new Trace(resp.trace, query[counter - 1]);
            tempTraces.push(trace);
          }

          isHistogramResult = isDisplayChart && isHistogramData(resp.data.result);
          seriesLimit = isHistogramResult ? Infinity : defaultLimit;
          const freeTempSize = seriesLimit - tempData.length;
          resp.data.result.slice(0, freeTempSize).forEach((d: MetricBase) => {
            d.group = counter;
            tempData.push(d);
          });

          totalLength += resp.data.result.length;
        } else {
          tempData.push({ metric: {}, values: [], group: counter } as MetricBase);
          setQueryErrors(prev => [...prev, `${resp.errorType}\r\n${resp?.error}`]);
        }
        counter++;
      }

      const limitText = `Showing ${tempData.length} series out of ${totalLength} series due to performance reasons. Please narrow down the query, so it returns less series`;
      setWarning(totalLength > seriesLimit ? limitText : "");
      isDisplayChart ? setGraphData(tempData as MetricResult[]) : setLiveData(tempData as InstantMetricResult[]);
      setTraces(tempTraces);
      setIsHistogram(prev => totalLength ? isHistogramResult : prev);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setIsLoading(false);
  };

  const throttledFetchData = useCallback(debounce(fetchData, 300), []);

  const fetchUrl = useMemo(() => {
    setError("");
    setQueryErrors([]);
    setQueryStats([]);
    const expr = predefinedQuery ?? query;
    const displayChart = (display || displayType) === "chart";
    if (!period) return;
    if (!serverUrl) {
      setError(ErrorTypes.emptyServer);
    } else if (expr.every(q => !q.trim())) {
      setQueryErrors(expr.map(() => ErrorTypes.validQuery));
    } else if (isValidHttpUrl(serverUrl)) {
      const updatedPeriod = { ...period };
      updatedPeriod.step = customStep;
      return expr.map(q => displayChart
        ? getQueryRangeUrl(serverUrl, q, updatedPeriod, nocache, isTracingEnabled)
        : getQueryUrl(serverUrl, q, updatedPeriod, nocache, isTracingEnabled));
    } else {
      setError(ErrorTypes.validServer);
    }
  },
  [serverUrl, period, displayType, customStep, hideQuery]);

  const [prevUrl, setPrevUrl] = useState<string[]>([]);

  useEffect(() => {
    const isLazyPredefined = (fetchUrl === prevUrl && !!predefinedQuery);
    if (!visible || !fetchUrl?.length || isLazyPredefined) return;
    setIsLoading(true);
    const expr = predefinedQuery ?? query;
    throttledFetchData({
      fetchUrl,
      fetchQueue,
      displayType: display || displayType,
      query: expr,
      stateSeriesLimits,
      showAllSeries,
      hideQuery,
    });
    setPrevUrl(fetchUrl);
  }, [fetchUrl, visible, stateSeriesLimits, showAllSeries]);

  useEffect(() => {
    const fetchPast = fetchQueue.slice(0, -1);
    if (!fetchPast.length) return;
    fetchPast.map(f => f.abort());
    setFetchQueue(fetchQueue.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  useEffect(() => {
    if (defaultStep === customStep) setGraphData([]);
  }, [isHistogram]);

  return { fetchUrl, isLoading, graphData, liveData, error, queryErrors, setQueryErrors, queryStats, warning, traces, isHistogram };
};
