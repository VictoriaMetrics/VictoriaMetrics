import {useEffect, useState} from "preact/compat";
import {getQueryOptions} from "../api/query-range";
import {useAppState} from "../state/common/StateContext";
import {getAppModeEnable, getAppModeParams} from "../utils/app-mode";

const appModeEnable = getAppModeEnable();
const {serverURL: appServerUrl} = getAppModeParams();

export const useFetchQueryOptions = (): {
  queryOptions: string[],
} => {
  const {serverUrl} = useAppState();

  const [queryOptions, setQueryOptions] = useState([]);

  const fetchOptions = async () => {
    const server = appModeEnable ? appServerUrl : serverUrl;
    if (!server) return;
    const url = getQueryOptions(server);

    try {
      const response = await fetch(url);
      const resp = await response.json();
      if (response.ok) {
        setQueryOptions(resp.data);
      }
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    fetchOptions();
  }, [serverUrl]);

  return { queryOptions };
};
