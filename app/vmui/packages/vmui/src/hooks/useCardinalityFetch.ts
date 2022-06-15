import {ErrorTypes} from "../types";
import {useAppState} from "../state/common/StateContext";
import {useEffect, useState} from "preact/compat";
import {CardinalityRequestsParams, getCardinalityInfo} from "../api/tsdb";
import {getAppModeEnable, getAppModeParams} from "../utils/app-mode";
import {TSDBStatus} from "../components/CardinalityPanel/types";
import {useCardinalityState} from "../state/cardinality/CardinalityStateContext";
import AppConfigurator from "../components/CardinalityPanel/appConfigurator";

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

export const useFetchQuery = (): {
  fetchUrl?: string[],
  isLoading: boolean,
  error?: ErrorTypes | string
  appConfigurator: AppConfigurator,
} => {
  const appConfigurator = new AppConfigurator();
  const {topN, extraLabel, match, date, runQuery, focusLabel} = useCardinalityState();

  const {serverUrl} = useAppState();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [tsdbStatus, setTSDBStatus] = useState<TSDBStatus>(appConfigurator.defaultTSDBStatus);

  useEffect(() => {
    if (error) {
      setTSDBStatus(appConfigurator.defaultTSDBStatus);
      setIsLoading(false);
    }
  }, [error]);

  const fetchCardinalityInfo = async (requestParams: CardinalityRequestsParams) => {
    const server = appModeEnable ? appServerUrl : serverUrl;
    if (!server) return;
    setError("");
    setIsLoading(true);
    setTSDBStatus(appConfigurator.defaultTSDBStatus);
    const url = getCardinalityInfo(server, requestParams);

    try {
      const response = await fetch(url);
      const resp = await response.json();
      if (response.ok) {
        const {data} = resp;
        setTSDBStatus({ ...data });
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
    fetchCardinalityInfo({topN, extraLabel, match, date, focusLabel});
  }, [serverUrl, runQuery, date]);

  appConfigurator.tsdbStatusData = tsdbStatus;
  return {isLoading, appConfigurator: appConfigurator, error};
};
