import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getJobsUrl } from "../../../api/explore-metrics";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";

interface FetchJobsReturn {
  jobs: string[],
  isLoading: boolean,
  error?: ErrorTypes | string,
}

export const useFetchJobs = (): FetchJobsReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [jobs, setJobs] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getJobsUrl(serverUrl, period), [serverUrl, period]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data || []) as string[];
        setJobs(data.sort((a, b) => a.localeCompare(b)));

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

  return { jobs, isLoading, error };
};
