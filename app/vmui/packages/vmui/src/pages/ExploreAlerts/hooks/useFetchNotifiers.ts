import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getNotifiersUrl, parseJsonResponse } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { Notifier, ErrorTypes } from "../../../types";

interface FetchNotifiersReturn {
  notifiers: Notifier[];
  isLoading: boolean;
  error?: ErrorTypes | string;
  // warnings is non-empty when some of the vmalerts at -vmalert.proxyURL are unavailable.
  // The rest of vmalerts still return their notifiers in this case.
  warnings: string[];
}

interface FetchNotifiersProps {
  // source limits the request to a single vmalert at -vmalert.proxyURL.
  source: string;
}

export const useFetchNotifiers = ({ source }: FetchNotifiersProps): FetchNotifiersReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [notifiers, setNotifiers] = useState<Notifier[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [warnings, setWarnings] = useState<string[]>([]);

  const fetchUrl = useMemo(() => getNotifiersUrl(serverUrl, source), [serverUrl, source]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await parseJsonResponse(response);

        if (response.ok) {
          const data = (resp.data.notifiers || []) as Notifier[];
          setNotifiers(data.sort((a, b) => a.kind.localeCompare(b.kind)));
          setWarnings((resp.warnings || []) as string[]);
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

  return { notifiers, isLoading, error, warnings };
};
