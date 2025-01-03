import { Dispatch, SetStateAction, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import { MetricBase, MetricResult, ExportMetricResult } from "../../../api/types";
import { ErrorTypes, SeriesLimits } from "../../../types";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useAppState } from "../../../state/common/StateContext";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { isValidHttpUrl } from "../../../utils/url";
import { getExportDataUrl } from "../../../api/query-range";
import { parseLineToJSON } from "../../../utils/json";

interface FetchQueryParams {
  hideQuery?: number[];
  showAllSeries?: boolean;
}

interface FetchQueryReturn {
  fetchUrl?: string[],
  isLoading: boolean,
  data?: MetricResult[],
  error?: ErrorTypes | string,
  queryErrors: (ErrorTypes | string)[],
  setQueryErrors: Dispatch<SetStateAction<string[]>>,
  warning?: string,
  abortFetch: () => void
}

export const useFetchExport = ({ hideQuery, showAllSeries }: FetchQueryParams): FetchQueryReturn => {
  const { query } = useQueryState();
  const { period } = useTimeState();
  const { displayType, reduceMemUsage, seriesLimits: stateSeriesLimits } = useCustomPanelState();
  const { serverUrl } = useAppState();

  const [isLoading, setIsLoading] = useState(false);
  const [data, setData] = useState<MetricResult[]>();
  const [error, setError] = useState<ErrorTypes | string>();
  const [queryErrors, setQueryErrors] = useState<string[]>([]);
  const [warning, setWarning] = useState<string>();

  const abortControllerRef = useRef(new AbortController());

  const fetchUrl = useMemo(() => {
    setError("");
    setQueryErrors([]);
    if (!period) return;
    if (!serverUrl) {
      setError(ErrorTypes.emptyServer);
    } else if (query.every(q => !q.trim())) {
      setQueryErrors(query.map(() => ErrorTypes.validQuery));
    } else if (isValidHttpUrl(serverUrl)) {
      const updatedPeriod = { ...period };
      return query.map(q => getExportDataUrl(serverUrl, q, updatedPeriod, reduceMemUsage));
    } else {
      setError(ErrorTypes.validServer);
    }
  }, [serverUrl, period, hideQuery, reduceMemUsage]);

  const fetchData = useCallback(async ({ fetchUrl, stateSeriesLimits, showAllSeries }: {
    fetchUrl: string[];
    stateSeriesLimits: SeriesLimits;
    showAllSeries?: boolean;
  }) => {
    abortControllerRef.current.abort();
    abortControllerRef.current = new AbortController();
    const { signal } = abortControllerRef.current;
    setIsLoading(true);
    try {
      const tempData: MetricBase[] = [];
      const seriesLimit = showAllSeries ? Infinity : +stateSeriesLimits[displayType] || Infinity;
      let counter = 1;
      let totalLength = 0;

      for await (const url of fetchUrl) {

        const isHideQuery = hideQuery?.includes(counter - 1);
        if (isHideQuery) {
          setQueryErrors(prev => [...prev, ""]);
          counter++;
          continue;
        }

        const response = await fetch(url, { signal });
        const text = await response.text();

        if (!response.ok || !response.body) {
          tempData.push({ metric: {}, values: [], group: counter } as MetricBase);
          setError(text);
          setQueryErrors(prev => [...prev, `${text}`]);
        } else {
          setQueryErrors(prev => [...prev, ""]);
          const freeTempSize = seriesLimit - tempData.length;
          const lines = text.split("\n").filter(line => line);
          const lineLimited = lines.slice(0, freeTempSize).sort();
          lineLimited.forEach((line: string) => {
            const jsonLine = parseLineToJSON(line) as (ExportMetricResult | null);
            if (!jsonLine) return;
            tempData.push({
              group: counter,
              metric: jsonLine.metric,
              values: jsonLine.values.map((value, index) => [(jsonLine.timestamps[index] / 1000), value]),
            } as MetricBase);
          });
          totalLength += lines.length;
        }

        counter++;
      }
      const limitText = `Showing ${tempData.length} series out of ${totalLength} series due to performance reasons. Please narrow down the query, so it returns less series`;
      setWarning(totalLength > seriesLimit ? limitText : "");
      setData(tempData as MetricResult[]);
      setIsLoading(false);
    } catch (e) {
      setIsLoading(false);
      if (e instanceof Error && e.name !== "AbortError") {
        setError(String(e));
        console.error(e);
      }
    }
  }, [displayType, hideQuery]);

  const abortFetch = useCallback(() => {
    abortControllerRef.current.abort();
    setData([]);
  }, [abortControllerRef]);

  useEffect(() => {
    if (!fetchUrl?.length) return;
    const timer = setTimeout(fetchData, 400, { fetchUrl, stateSeriesLimits, showAllSeries });
    return () => {
      abortControllerRef.current?.abort();
      clearTimeout(timer);
    };
  }, [fetchUrl, stateSeriesLimits, showAllSeries]);

  return {
    fetchUrl,
    isLoading,
    data,
    error,
    queryErrors,
    setQueryErrors,
    warning,
    abortFetch,
  };
};
