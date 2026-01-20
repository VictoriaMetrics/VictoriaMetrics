import { Dispatch, SetStateAction, useCallback, useEffect, useMemo, useState } from "preact/compat";
import { getQueryRangeUrl, getQueryUrl } from "../api/query-range";
import { useAppState } from "../state/common/StateContext";
import { InstantMetricResult, MetricBase, MetricResult, QueryStats } from "../api/types";
import { isValidHttpUrl } from "../utils/url";
import { DisplayType, ErrorTypes, SeriesLimits } from "../types";
import debounce from "lodash.debounce";
import Trace from "../components/TraceQuery/Trace";
import { useQueryState } from "../state/query/QueryStateContext";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCustomPanelState } from "../state/customPanel/CustomPanelStateContext";
import { isHistogramData } from "../utils/metric";
import { useGraphState } from "../state/graph/GraphStateContext";
import { getStepFromDuration } from "../utils/time";
import { getQueryStringValue } from "../utils/query-string";

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
  setQueryErrors: Dispatch<SetStateAction<string[]>>,
  queryStats: QueryStats[],
  warning?: string,
  traces?: Trace[],
  isHistogram: boolean,
  abortFetch: () => void
}

interface FetchDataParams {
  fetchUrl: string[],
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
  showAllSeries,
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
    return getStepFromDuration(end - start, isHistogramState, displayType);
  }, [period, isHistogramState, displayType]);

  const fetchData = async ({
    fetchUrl,
    displayType,
    query,
    stateSeriesLimits,
    showAllSeries,
    hideQuery,
  }: FetchDataParams) => {

    const controller = new AbortController();
    setFetchQueue(prev => [...prev, controller]);

    try {
      const isDisplayChart = displayType === DisplayType.chart;
      const defaultLimit = showAllSeries ? Infinity : (+stateSeriesLimits[displayType] || Infinity);
      let seriesLimit = defaultLimit;
      const tempData: MetricBase[] = [];
      const tempTraces: Trace[] = [];
      const tempStats: QueryStats[] = [];
      const tempErrors: string[] = [];

      let counter = 1;
      let totalLength = 0;
      let isHistogramResult = false;

      for await (const url of fetchUrl) {

        const isHideQuery = hideQuery?.includes(counter - 1);
        if (isHideQuery) {
          tempErrors.push("");
          tempStats.push({});
          counter++;
          continue;
        }

        const urlObj = new URL(url);
        const response = await fetch(`${urlObj.origin}${urlObj.pathname}`, {
          signal: controller.signal,
          method: "POST",
          body: urlObj.searchParams
        });
        const resp = await response.json();

        if (response.ok) {
          tempStats.push({
            ...resp?.stats,
            isPartial: resp?.isPartial,
            resultLength: resp.data.result.length,
          });
          tempErrors.push("");

          if (resp.trace) {
            const trace = new Trace(resp.trace, query[counter - 1]);
            tempTraces.push(trace);
          }

          const preventChangeType = !!getQueryStringValue("display_mode", null);
          isHistogramResult = isDisplayChart && !preventChangeType && isHistogramData(resp.data.result);
          seriesLimit = isHistogramResult ? Infinity : defaultLimit;
          const freeTempSize = Math.max(0, seriesLimit - tempData.length);
          resp.data.result.slice(0, freeTempSize).forEach((d: MetricBase) => {
            d.group = counter;
            tempData.push(d);
          });

          totalLength += resp.data.result.length;
        } else {
          tempData.push({ metric: {}, values: [], group: counter } as MetricBase);
          const errorType = resp.errorType || ErrorTypes.unknownType;
          const errorMessage = resp?.error || resp?.message || "see console for more details";
          const error = [errorType, errorMessage].join(",\r\n");
          tempErrors.push(error);
          console.error(`Fetch query error: ${errorType}`, resp);
        }
        counter++;
      }
      setQueryErrors(tempErrors);
      setQueryStats(tempStats);

      const shownSeries = tempData.length;
      setWarning(shownSeries < totalLength
        ? `Showing ${shownSeries} series out of ${totalLength} series due to performance reasons. Please narrow down the query, so it returns fewer series`
        : ""
      );

      isDisplayChart ? setGraphData(tempData as MetricResult[]) : setLiveData(tempData as InstantMetricResult[]);
      setTraces(tempTraces);
      setIsHistogram(prev => totalLength ? isHistogramResult : prev);
    } catch (e) {
      const error = e as Error;
      if (error.name === "AbortError") {
        // Aborts are expected, don't show an error for them.
        setIsLoading(false);
        return;
      }
      const helperText = "Please check your serverURL settings and confirm server availability.";
      let text = `Error executing query: ${error.message}. ${helperText}`;
      if (error.message === "Unexpected end of JSON input") {
        text += "\nAdditionally, this error can occur if the server response is too large to process. Apply more specific filters to reduce the data volume.";
      }
      setError(text);
    }
    setIsLoading(false);
  };

  const throttledFetchData = useCallback(debounce(fetchData, 300), []);

  const fetchUrl = useMemo(() => {
    const expr = (predefinedQuery ?? query).filter(Boolean);
    const displayChart = (display || displayType) === DisplayType.chart;

    if (!period || !serverUrl || !isValidHttpUrl(serverUrl) || !expr.length) return;

    const updatedPeriod = { ...period, step: customStep };
    return expr.map(q => displayChart
      ? getQueryRangeUrl(serverUrl, q, updatedPeriod, nocache, isTracingEnabled)
      : getQueryUrl(serverUrl, q, updatedPeriod, nocache, isTracingEnabled));
  }, [serverUrl, period, displayType, customStep, nocache, isTracingEnabled, display, predefinedQuery, query]);

  const abortFetch = useCallback(() => {
    fetchQueue.forEach(f => f.abort());

    setFetchQueue([]);
    setGraphData([]);
    setLiveData([]);
  }, [fetchQueue]);

  const [prevUrl, setPrevUrl] = useState<string[]>([]);

  useEffect(() => {
    const isLazyPredefined = (fetchUrl === prevUrl && !!predefinedQuery);
    if (!visible || !fetchUrl?.length || isLazyPredefined) return;
    setIsLoading(true);
    const expr = predefinedQuery ?? query;
    throttledFetchData({
      fetchUrl,
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

    setFetchQueue(prev => prev.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  useEffect(() => {
    if (defaultStep === customStep) setGraphData([]);
  }, [isHistogram]);

  useEffect(() => {
    setError("");
    setQueryErrors([]);
    setQueryStats([]);

    const expr = predefinedQuery ?? query;
    if (!period) return;
    if (!serverUrl) { setError(ErrorTypes.emptyServer); return; }
    if (!isValidHttpUrl(serverUrl)) { setError(ErrorTypes.validServer); return; }
    if (expr.every(q => !q.trim())) { setQueryErrors(expr.map(() => ErrorTypes.validQuery)); }
  }, [serverUrl, period, displayType, customStep, nocache, isTracingEnabled, display, predefinedQuery, query]);

  return {
    fetchUrl,
    isLoading,
    graphData,
    liveData,
    error,
    queryErrors,
    setQueryErrors,
    queryStats,
    warning,
    traces,
    isHistogram,
    abortFetch,
  };
};
