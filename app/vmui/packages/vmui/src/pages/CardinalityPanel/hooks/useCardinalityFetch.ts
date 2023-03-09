import { ErrorTypes } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useEffect, useState } from "preact/compat";
import { CardinalityRequestsParams, getCardinalityInfo } from "../../../api/tsdb";
import { TSDBStatus } from "../types";
import AppConfigurator from "../appConfigurator";
import { useSearchParams } from "react-router-dom";
import dayjs from "dayjs";
import { DATE_FORMAT } from "../../../constants/date";

export const useFetchQuery = (): {
  fetchUrl?: string[],
  isLoading: boolean,
  error?: ErrorTypes | string
  appConfigurator: AppConfigurator,
} => {
  const appConfigurator = new AppConfigurator();

  const [searchParams] = useSearchParams();
  const match = searchParams.get("match");
  const focusLabel = searchParams.get("focusLabel");
  const topN = +(searchParams.get("topN") || 10);
  const date = searchParams.get("date") || dayjs().tz().format(DATE_FORMAT);

  const { serverUrl } = useAppState();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [tsdbStatus, setTSDBStatus] = useState<TSDBStatus>(appConfigurator.defaultTSDBStatus);

  const fetchCardinalityInfo = async (requestParams: CardinalityRequestsParams) => {
    if (!serverUrl) return;
    setError("");
    setIsLoading(true);
    setTSDBStatus(appConfigurator.defaultTSDBStatus);

    const defaultParams = { date: requestParams.date, topN: 0, match: "", focusLabel: "" } as CardinalityRequestsParams;
    const url = getCardinalityInfo(serverUrl, requestParams);
    const urlDefault = getCardinalityInfo(serverUrl, defaultParams);

    try {
      const response = await fetch(url);
      const resp = await response.json();
      const responseTotal = await fetch(urlDefault);
      const respTotals = await responseTotal.json();
      if (response.ok) {
        const { data } = resp;
        const { totalSeries } = respTotals.data;
        const result = { ...data } as TSDBStatus;
        result.totalSeriesByAll = totalSeries;

        const name = match?.replace(/[{}"]/g, "");
        result.seriesCountByLabelValuePair = result.seriesCountByLabelValuePair.filter(s => s.name !== name);

        setTSDBStatus(result);
        setIsLoading(false);
      } else {
        setError(resp.error);
        setTSDBStatus(appConfigurator.defaultTSDBStatus);
        setIsLoading(false);
      }
    } catch (e) {
      setIsLoading(false);
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
    }
  };


  useEffect(() => {
    fetchCardinalityInfo({ topN, match, date, focusLabel });
  }, [serverUrl, match, focusLabel, topN, date]);

  useEffect(() => {
    if (error) {
      setTSDBStatus(appConfigurator.defaultTSDBStatus);
      setIsLoading(false);
    }
  }, [error]);

  appConfigurator.tsdbStatusData = tsdbStatus;
  return { isLoading, appConfigurator: appConfigurator, error };
};
