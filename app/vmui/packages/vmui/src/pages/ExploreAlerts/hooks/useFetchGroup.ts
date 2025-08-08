import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getGroupUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";

interface FetchGroupReturn<T> {
  group?: T;
  isLoading: boolean;
  error?: ErrorTypes | string;
}

interface FetchGroupProps {
  id: string;
}

export const useFetchGroup = <T>({
  id,
}: FetchGroupProps): FetchGroupReturn<T> => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [group, setGroup] = useState<T>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getGroupUrl(serverUrl, id),
    [serverUrl, id],
  );

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();

        if (response.ok) {
          setGroup(resp as T);
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

  return { group, isLoading, error };
};
