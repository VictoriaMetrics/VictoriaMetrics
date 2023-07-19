import { useEffect, useMemo, useState } from "preact/compat";
import { getActiveQueries } from "../../../api/active-queries";
import { useAppState } from "../../../state/common/StateContext";
import { ActiveQueriesType, ErrorTypes } from "../../../types";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../../constants/date";

interface FetchActiveQueries {
  data: ActiveQueriesType[];
  isLoading: boolean;
  lastUpdated: string;
  error?: ErrorTypes | string;
  fetchData: () => Promise<void>;
}

export const useFetchActiveQueries = (): FetchActiveQueries => {
  const { serverUrl } = useAppState();

  const [activeQueries, setActiveQueries] = useState<ActiveQueriesType[]>([]);
  const [lastUpdated, setLastUpdated] = useState<string>(dayjs().format(DATE_FULL_TIMEZONE_FORMAT));
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getActiveQueries(serverUrl), [serverUrl]);

  const fetchData = async () => {
    setIsLoading(true);
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();
      setActiveQueries(resp.data);
      setLastUpdated(dayjs().format("HH:mm:ss:SSS"));
      if (response.ok) {
        setError(undefined);
      } else {
        setError(`${resp.errorType}\r\n${resp?.error}`);
      }
    } catch (e) {
      if (e instanceof Error) {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setIsLoading(false);
  };

  useEffect(() => {
    fetchData().catch(console.error);
  }, [fetchUrl]);

  return { data: activeQueries, lastUpdated, isLoading, error, fetchData };
};
