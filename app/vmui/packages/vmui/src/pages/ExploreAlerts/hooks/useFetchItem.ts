import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getItemUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";

interface FetchItemReturn<T> {
  item?: T;
  isLoading: boolean;
  error?: ErrorTypes | string;
}

interface FetchItemProps {
  groupId: string;
  id: string;
  mode: string;
}

export const useFetchItem = <T>({
  groupId,
  id,
  mode,
}: FetchItemProps): FetchItemReturn<T> => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [item, setItem] = useState<T>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getItemUrl(serverUrl, groupId, id, mode),
    [serverUrl, groupId, id, mode],
  );

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();

        if (response.ok) {
          setItem(resp as T);
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

  return { item, isLoading, error };
};
