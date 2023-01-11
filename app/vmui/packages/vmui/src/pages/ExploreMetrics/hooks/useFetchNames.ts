import { useEffect, useMemo, useState } from "preact/compat";
import { getNamesUrl } from "../../../api/explore-metrics";
import { useAppState } from "../../../state/common/StateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { ErrorTypes } from "../../../types";

interface FetchNamesReturn {
  names: string[],
  isLoading: boolean,
  error?: ErrorTypes | string,
}

export const useFetchNames = (job: string, instance: string): FetchNamesReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [names, setNames] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getNamesUrl(serverUrl, period, job, instance), [serverUrl, period, job, instance]);

  useEffect(() => {
    if (!job) return;
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data || []) as string[];
        setNames(data.sort((a, b) => a.localeCompare(b)));

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

  return { names, isLoading, error };
};
