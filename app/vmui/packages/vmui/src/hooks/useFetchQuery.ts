import { useCallback, useEffect, useMemo, useState } from "preact/compat";
import { getQueryRangeUrl, getQueryUrl } from "../api/query-range";
import { useAppState } from "../state/common/StateContext";
import { InstantMetricResult, MetricBase, MetricResult } from "../api/types";
import { isValidHttpUrl } from "../utils/url";
import { ErrorTypes, SeriesLimits } from "../types";
import debounce from "lodash.debounce";
import { DisplayType } from "../pages/CustomPanel/DisplayTypeSwitch";
import Trace from "../components/TraceQuery/Trace";
import { useQueryState } from "../state/query/QueryStateContext";
import { useTimeState } from "../state/time/TimeStateContext";
import { useCustomPanelState } from "../state/customPanel/CustomPanelStateContext";

interface FetchQueryParams {
  predefinedQuery?: string[]
  visible: boolean
  display?: DisplayType,
  customStep: number,
  hideQuery?: number[]
  showAllSeries?: boolean
}

interface FetchQueryReturn {
  fetchUrl?: string[],
  isLoading: boolean,
  graphData?: MetricResult[],
  liveData?: InstantMetricResult[],
  error?: ErrorTypes | string,
  warning?: string,
  traces?: Trace[],
}

interface FetchDataParams {
  fetchUrl: string[],
  fetchQueue: AbortController[],
  displayType: DisplayType,
  query: string[],
  stateSeriesLimits: SeriesLimits,
  showAllSeries?: boolean,
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

  const fetchData = async ({
    fetchUrl,
    fetchQueue,
    displayType,
    query,
    stateSeriesLimits,
    showAllSeries,
  }: FetchDataParams) => {
    const controller = new AbortController();
    setFetchQueue([...fetchQueue, controller]);
    try {
      const isDisplayChart = displayType === "chart";
      const seriesLimit = showAllSeries ? Infinity : stateSeriesLimits[displayType];
      const tempData: MetricBase[] = [];
      const tempTraces: Trace[] = [];
      let counter = 1;
      let totalLength = 0;

      for await (const url of fetchUrl) {
        const response = await fetch(url, { signal: controller.signal });
        const resp = await response.json();

        if (response.ok) {
          setError(undefined);

          if (resp.trace) {
            const trace = new Trace(resp.trace, query[counter - 1]);
            tempTraces.push(trace);
          }

          const freeTempSize = seriesLimit - tempData.length;
          resp.data.result.slice(0, freeTempSize).forEach((d: MetricBase) => {
            d.group = counter;
            tempData.push(d);
          });

          totalLength += resp.data.result.length;
          counter++;
        } else {
          setError(`${resp.errorType}\r\n${resp?.error}`);
        }
      }

      const limitText = `Showing ${seriesLimit} series out of ${totalLength} series due to performance reasons. Please narrow down the query, so it returns less series`;
      setWarning(totalLength > seriesLimit ? limitText : "");

      isDisplayChart ? setGraphData(tempData as MetricResult[]) : setLiveData(tempData as InstantMetricResult[]);
      setTraces(tempTraces);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setIsLoading(false);
  };

  const throttledFetchData = useCallback(debounce(fetchData, 800), []);

  const filterExpr = (q: string, i: number) => {
    const byQuery = q.trim();
    const byHideQuery = hideQuery ? !hideQuery.includes(i) : true;
    return byQuery && byHideQuery;
  };

  const fetchUrl = useMemo(() => {
    const expr = predefinedQuery ?? query;
    const displayChart = (display || displayType) === "chart";
    if (!period) return;
    if (!serverUrl) {
      setError(ErrorTypes.emptyServer);
    } else if (expr.every(q => !q.trim())) {
      setError(ErrorTypes.validQuery);
    } else if (isValidHttpUrl(serverUrl)) {
      const updatedPeriod = { ...period };
      updatedPeriod.step = customStep;
      return expr.filter(filterExpr).map(q => displayChart
        ? getQueryRangeUrl(serverUrl, q, updatedPeriod, nocache, isTracingEnabled)
        : getQueryUrl(serverUrl, q, updatedPeriod, isTracingEnabled));
    } else {
      setError(ErrorTypes.validServer);
    }
  },
  [serverUrl, period, displayType, customStep, hideQuery]);

  useEffect(() => {
    if (!visible || !fetchUrl?.length) return;
    setIsLoading(true);
    const expr = predefinedQuery ?? query;
    throttledFetchData({
      fetchUrl,
      fetchQueue,
      displayType: display || displayType,
      query: expr,
      stateSeriesLimits,
      showAllSeries,
    });
  }, [fetchUrl, visible, stateSeriesLimits, showAllSeries]);

  useEffect(() => {
    const fetchPast = fetchQueue.slice(0, -1);
    if (!fetchPast.length) return;
    fetchPast.map(f => f.abort());
    setFetchQueue(fetchQueue.filter(f => !f.signal.aborted));
  }, [fetchQueue]);

  return { fetchUrl, isLoading, graphData, liveData, error, warning, traces };
};
