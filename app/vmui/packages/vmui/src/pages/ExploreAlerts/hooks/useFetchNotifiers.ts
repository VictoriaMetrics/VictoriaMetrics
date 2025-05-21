import { useEffect, useMemo, useState } from "preact/compat";
import { getNotifiersUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";

export const useFetchNotifiers = (): Notifier[] => {
  const { serverUrl } = useAppState();

  const [notifiers, setNotifiers] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getNotifiersUrl(serverUrl), [serverUrl]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data.notifiers || []) as T[];
        setNotifiers(data.sort((a, b) => a.name.localeCompare(b.name)));

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

    fetchData().catch(console.error);
  }, [fetchUrl]);

  return { notifiers, isLoading, error };
};
