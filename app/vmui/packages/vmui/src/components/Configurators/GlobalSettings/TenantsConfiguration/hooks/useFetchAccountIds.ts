import { useAppState } from "../../../../../state/common/StateContext";
import { useMemo, useState, useRef, useCallback, useEffect } from "preact/compat";
import { ErrorTypes } from "../../../../../types";
import { getAccountIds } from "../../../../../api/accountId";
import { useTimeState } from "../../../../../state/time/TimeStateContext";
import { routerOptions } from "../../../../../router";
import { useSearchParams } from "react-router";
import { useLocation } from "react-router-dom";

const collator = new Intl.Collator();

export const useFetchAccountIds = () => {
  const { pathname } = useLocation();
  const { serverUrl } = useAppState();
  const { period } = useTimeState();
  const [searchParams] = useSearchParams();

  const route = routerOptions[pathname];
  const tenantPeriod = route?.getTenantPeriod?.({ searchParams }) ?? period;

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [accountIds, setAccountIds] = useState<string[]>([]);
  const lastRequestKey = useRef<string>();
  const abortControllerRef = useRef<AbortController>();

  const fetchUrl = useMemo(() => getAccountIds(serverUrl, tenantPeriod), [serverUrl, tenantPeriod]);

  const fetchAccountIds = useCallback(async () => {
    setError(undefined);
    abortControllerRef.current?.abort();

    if (lastRequestKey.current === fetchUrl) return;

    setIsLoading(true);

    const currentAbortController = new AbortController();
    abortControllerRef.current = currentAbortController;

    try {
      const response = await fetch(fetchUrl, { signal: currentAbortController.signal });
      const resp = await response.json();

      if (currentAbortController.signal.aborted) return;

      if (response.ok) {
        const data = (resp.data || []) as string[];
        setAccountIds(data.sort(collator.compare));
        lastRequestKey.current = fetchUrl;
      } else {
        setError(`${resp.errorType}\r\n${resp?.error}`);
      }
    } catch (e) {
      if (!currentAbortController.signal.aborted && e instanceof Error) {
        setError(`${e.name}: ${e.message}`);
      }
    } finally {
      if (abortControllerRef.current === currentAbortController) {
        abortControllerRef.current = undefined;
        setIsLoading(false);
      }
    }
  }, [fetchUrl]);

  const cancelFetchAccountIds = useCallback(() => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = undefined;
    setIsLoading(false);
  }, []);

  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
      abortControllerRef.current = undefined;
    };
  }, []);

  return {
    accountIds,
    isLoading,
    error,
    fetchAccountIds,
    cancelFetchAccountIds
  };
};
