import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getNotifiersUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { Notifier, ErrorTypes } from "../../../types";

interface FetchNotifiersReturn {
  notifiers: Notifier[];
  isLoading: boolean;
  error?: ErrorTypes | string;
}

export const useFetchNotifiers = (): FetchNotifiersReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [notifiers, setNotifiers] = useState<Notifier[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getNotifiersUrl(serverUrl), [serverUrl]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();

        if (response.ok) {
          const data = (resp.data.notifiers || []) as Notifier[];
          setNotifiers(data.sort((a, b) => a.kind.localeCompare(b.kind)));
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

    fetchData().catch(console.error);
  }, [fetchUrl, period]);

  return { notifiers, isLoading, error };
};
