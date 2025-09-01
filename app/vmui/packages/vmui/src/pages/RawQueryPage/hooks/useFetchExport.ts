import { Dispatch, SetStateAction, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import { MetricBase, MetricResult, ExportMetricResult } from "../../../api/types";
import { ErrorTypes, SeriesLimits, TimeParams } from "../../../types";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useAppState } from "../../../state/common/StateContext";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { isValidHttpUrl } from "../../../utils/url";
import { getExportCSVDataUrl, getExportDataUrl, getExportJSONDataUrl } from "../../../api/query-range";
import { parseLineToJSON } from "../../../utils/json";
import { downloadCSV, downloadJSON } from "../../../utils/file";
import { useSnack } from "../../../contexts/Snackbar";

interface FetchQueryParams {
  hideQuery?: number[];
  showAllSeries?: boolean;
}

interface FetchQueryReturn {
  fetchUrl?: string[],
  exportData: (format: ExportFormats) => void,
  isLoading: boolean,
  data?: MetricResult[],
  error?: ErrorTypes | string,
  queryErrors: (ErrorTypes | string)[],
  setQueryErrors: Dispatch<SetStateAction<string[]>>,
  warning?: string,
  abortFetch: () => void
}

type ExportFormats = "csv" | "json";
type FormatDownloader = (serverUrl: string, query: string[], period: TimeParams, reduceMemUsage: boolean) => void;
type DownloadFileFormats = Record<ExportFormats, FormatDownloader>

export const useFetchExport = ({ hideQuery, showAllSeries }: FetchQueryParams): FetchQueryReturn => {
  const { query } = useQueryState();
  const { period } = useTimeState();
  const { displayType, reduceMemUsage, seriesLimits: stateSeriesLimits } = useCustomPanelState();
  const { serverUrl } = useAppState();
  const { showInfoMessage } = useSnack();

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

  const fileDownloaders: DownloadFileFormats = useMemo(() => {
    const getFilename = (format: ExportFormats) => `vmui_export_${query.join("_")}_${period.start}_${period.end}.${format}`;
    return {
      csv: async () => {
        const url = getExportCSVDataUrl(serverUrl, query, period, reduceMemUsage);
        const response = await fetch(url);
        try {
          let text = await response.text();
          text = "name,value,timestamp\n" + text;
          downloadCSV(text, getFilename("csv"));
        } catch (e) {
          console.error(e);
          showInfoMessage({ text: "Couldn't fetch data for CSV export. Please try again", type: "error" });
        }
      },
      json: async () => {
        const url = getExportJSONDataUrl(serverUrl, query, period, reduceMemUsage);
        try {
          const response = await fetch(url);
          const text = await response.text();
          downloadJSON(text, getFilename("json"));
        } catch (e) {
          console.error(e);
          showInfoMessage({ text: "Couldn't fetch data for JSON export. Please try again", type: "error" });
        }
      }
    };
  }, [query, period, serverUrl, reduceMemUsage]);

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
      const maxPointsPerSeries = Math.floor(window.innerWidth / 4);
      const maxResponseSize = 30 * 1024 * 1024; // 30 MiB

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
        const sizeInBytes = new TextEncoder().encode(text).length;

        if (sizeInBytes > maxResponseSize) {
          const errorMessage = "Response too large to display (over 30 MiB). Please narrow your query.";
          setError(errorMessage);
          setQueryErrors(prev => [...prev, errorMessage]);
          continue;
        }

        if (!response.ok || !response.body) {
          tempData.push({ metric: {}, values: [], group: counter } as MetricBase);
          setError(text);
          setQueryErrors(prev => [...prev, `${text}`]);
        } else {
          setQueryErrors(prev => [...prev, ""]);
          const freeTempSize = seriesLimit - tempData.length;
          const lines = text.split("\n").filter(line => line);
          const lineLimited = lines.slice(0, freeTempSize).sort();

          for (const line of lineLimited) {
            const jsonLine = parseLineToJSON(line) as ExportMetricResult | null;
            if (!jsonLine) continue;

            const { values: rawValues, timestamps: rawTimestamps } = jsonLine;
            const totalPoints = rawValues.length;

            const shouldDownsample = totalPoints > maxPointsPerSeries;
            const pointsToTake = shouldDownsample ? maxPointsPerSeries : totalPoints;
            const step = shouldDownsample ? totalPoints / maxPointsPerSeries : 1;

            const values: [number, number][] = Array.from({ length: pointsToTake }, (_, i) => {
              const idx = shouldDownsample ? Math.floor(i * step) : i;
              return [rawTimestamps[idx] / 1000, rawValues[idx]];
            });

            tempData.push({
              group: counter,
              metric: jsonLine.metric,
              values,
            } as MetricBase);
          }

          totalLength += lines.length;
        }

        counter++;
      }
      const limitText = `Showing ${tempData.length} series out of ${totalLength} series due to performance reasons. Please narrow down the query, so it returns fewer series`;
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

  const exportData = useCallback((format: ExportFormats) => {
    if (error) return;
    const updatedPeriod = { ...period };
    fileDownloaders[format](serverUrl, query, updatedPeriod, reduceMemUsage);
  }, [serverUrl, query, period, reduceMemUsage, error, fileDownloaders]);

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
    exportData
  };
};
