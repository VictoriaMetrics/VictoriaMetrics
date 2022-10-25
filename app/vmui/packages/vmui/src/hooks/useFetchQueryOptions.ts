import {useEffect, useState} from "preact/compat";
import {getQueryOptions} from "../api/query-range";
import {useAppState} from "../state/common/StateContext";

export const useFetchQueryOptions = (): {
  queryOptions: string[],
} => {
  const {serverUrl} = useAppState();

  const [queryOptions, setQueryOptions] = useState([]);

  const fetchOptions = async () => {
    if (!serverUrl) return;
    const url = getQueryOptions(serverUrl);

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
