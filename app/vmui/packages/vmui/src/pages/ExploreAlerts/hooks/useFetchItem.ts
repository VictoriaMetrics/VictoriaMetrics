import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getItemUrl, parseJsonResponse } from "../../../api/explore-alerts";
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
  // source is the name of the vmalert owning the item. See getVMAlertSource().
  source: string;
}

export const useFetchItem = <T>({
  groupId,
  id,
  mode,
  source,
}: FetchItemProps): FetchItemReturn<T> => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [item, setItem] = useState<T>();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getItemUrl(serverUrl, groupId, id, mode, source),
    [serverUrl, groupId, id, mode, source],
  );

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await parseJsonResponse(response);

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
