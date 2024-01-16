import { useMemo, useState } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";
import { useEffect } from "react";
import { MetricBase } from "../../../api/types";
import { useTimeState } from "../../../state/time/TimeStateContext";
import dayjs from "dayjs";

// TODO: Change the method of retrieving aliases from the configuration after the API has been added
const seriesQuery = `{
  for!="",
  __name__!~".*yhat.*|.*trend.*|.*anomaly_score.*|.*daily.*|.*additive_terms.*|.*multiplicative_terms.*|.*weekly.*"
}`;

export const useFetchAnomalySeries = () => {
  const { serverUrl } = useAppState();
  const { period: { start, end } } = useTimeState();

  const [series, setSeries] = useState<Record<string, MetricBase["metric"][]>>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  // TODO add cached metrics by date
  const fetchUrl = useMemo(() => {
    const startDay = dayjs(start * 1000).startOf("day").valueOf() / 1000;
    const endDay = dayjs(end * 1000).endOf("day").valueOf() / 1000;

    const params = new URLSearchParams({
      "match[]": seriesQuery,
      start: `${startDay}`,
      end: `${endDay}`
    });

    return `${serverUrl}/api/v1/series?${params}`;
  }, [serverUrl, start, end]);

  useEffect(() => {
    const fetchSeries = async () => {
      setError("");
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp?.data || []) as MetricBase["metric"][];
        const groupedByFor = data.reduce<{ [key: string]: MetricBase["metric"][] }>((acc, item) => {
          const forKey = item["for"];
          if (!acc[forKey]) acc[forKey] = [];
          acc[forKey].push(item);
          return acc;
        }, {});
        setSeries(groupedByFor);

        if (!response.ok) {
          const errorType = resp.errorType ? `${resp.errorType}\r\n` : "";
          setError(`${errorType}${resp?.error || resp?.message}`);
        }
      } catch (e) {
        if (e instanceof Error && e.name !== "AbortError") {
          const message = e.name === "SyntaxError" ? ErrorTypes.unknownType : `${e.name}: ${e.message}`;
          setError(`${message}`);
        }
      } finally {
        setIsLoading(false);
      }
    };

    fetchSeries();
  }, [fetchUrl]);

  return {
    error,
    series,
    isLoading,
  };
};
