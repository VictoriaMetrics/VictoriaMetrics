import { useTimeState } from "../../../state/time/TimeStateContext";
import { useEffect, useMemo, useState } from "preact/compat";
import { getGroupsUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes, Group } from "../../../types";

interface FetchGroupsReturn {
  groups: Group[];
  isLoading: boolean;
  error?: ErrorTypes | string;
}

interface FetchGroupsProps {
  blockFetch: boolean
}

export const useFetchGroups = ({ blockFetch }: FetchGroupsProps): FetchGroupsReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getGroupsUrl(serverUrl),
    [serverUrl],
  );

  const loaded = !!groups.length || !blockFetch;

  useEffect(() => {
    if (blockFetch) return;
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();

        if (response.ok) {
          const data = (resp.data.groups || []) as Group[];
          setGroups(data.sort((a, b) => a.name.localeCompare(b.name)));
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
  }, [fetchUrl, period, loaded]);

  return { groups, isLoading, error };
};
