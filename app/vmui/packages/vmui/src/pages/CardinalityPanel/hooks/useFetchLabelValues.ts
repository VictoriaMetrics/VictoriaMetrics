import { ErrorTypes } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getValuesUrl } from "../../../api/explore-metrics";

interface ValuesResponse {
  values: string[],
  isLoading: boolean,
  error?: ErrorTypes | string,
}

export const useFetchValues = (focusLabel: string): ValuesResponse => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [values, setValues] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getValuesUrl(serverUrl, period, focusLabel), [serverUrl, period, focusLabel]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data || []) as string[];

        setValues(data);

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

  return { values, isLoading, error };
};
