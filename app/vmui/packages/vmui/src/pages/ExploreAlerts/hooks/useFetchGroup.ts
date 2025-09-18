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
        switch (response.headers.get("Content-Type")) {
          case "application/json": {
            const resp = await response.json();
            if (response.ok) {
              setGroup(resp as T);
              setError(undefined);
            } else {
              setError(`${resp.errorType}\r\n${resp?.error}`);
            }
            break;
          }
          default: {
            let err = await response.text();
            if (err.startsWith("unsupported path requested")) {
              err = `Failed to show group details. Request to ${fetchUrl} failed with error:  ${err.trim()}.\nMake sure that vmalert is reachable at ${fetchUrl} and is of the same or higher version than vmselect`;
            } else {
              err = `${response.statusText}\r\n${err}`;
            }
            setError(err);
            break;
          }
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
