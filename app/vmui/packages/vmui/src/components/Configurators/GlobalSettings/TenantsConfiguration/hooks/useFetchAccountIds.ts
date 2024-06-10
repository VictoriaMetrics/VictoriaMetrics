import { useAppState } from "../../../../../state/common/StateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { ErrorTypes } from "../../../../../types";
import { getAccountIds } from "../../../../../api/accountId";
import { getAppModeEnable, getAppModeParams } from "../../../../../utils/app-mode";
import { getTenantIdFromUrl } from "../../../../../utils/tenants";

export const useFetchAccountIds = () => {
  const { useTenantID } = getAppModeParams();
  const appModeEnable = getAppModeEnable();
  const { serverUrl } = useAppState();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [accountIds, setAccountIds] = useState<string[]>([]);

  const fetchUrl = useMemo(() => getAccountIds(serverUrl), [serverUrl]);
  const isServerUrlWithTenant = useMemo(() => !!getTenantIdFromUrl(serverUrl), [serverUrl]);
  const preventFetch = appModeEnable ? !useTenantID : !isServerUrlWithTenant;

  useEffect(() => {
    if (preventFetch) return;
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data || []) as string[];
        setAccountIds(data.sort((a, b) => a.localeCompare(b)));

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

  return { accountIds, isLoading, error };
};
