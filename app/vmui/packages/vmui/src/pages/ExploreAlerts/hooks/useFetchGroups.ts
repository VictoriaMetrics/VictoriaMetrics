import { useEffect, useMemo, useState } from "preact/compat";
import { getGroupsUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes, Group } from "../../../types";

export const useFetchGroups = (): Group[] => {
  const { serverUrl } = useAppState();

  const [groups, setGroups] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(() => getGroupsUrl(serverUrl), [serverUrl]);

  useEffect(() => {
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(fetchUrl);
        const resp = await response.json();
        const data = (resp.data.groups || []) as T[];
        setGroups(data.sort((a, b) => a.name.localeCompare(b.name)));

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

  return { groups, isLoading, error };
};
