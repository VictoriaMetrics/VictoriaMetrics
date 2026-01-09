import { useMemo, useEffect, useState } from "preact/compat";
import { getGroupsUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes, Group } from "../../../types";
import { useTimeState } from "../../../state/time/TimeStateContext";

interface FetchGroupsReturn {
  groups: Group[];
  isLoading: boolean;
  error?: ErrorTypes | string;
  pageInfo: PageInfo;
}

interface FetchGroupsProps {
  blockFetch: boolean;
  search: string;
  ruleType: string;
  states: string[];
  pageNum: number;
  onPageChange: (num: number) => () => void;
}

interface PageInfo {
  page: number;
  total_pages: number;
  total_groups: number;
  total_rules: number;
}

const MAX_GROUPS = 100;

export const useFetchGroups = ({ blockFetch, pageNum, search, ruleType, states, onPageChange }: FetchGroupsProps): FetchGroupsReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: pageNum,
    total_pages: 1,
    total_groups: 0,
    total_rules: 0,
  });
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getGroupsUrl(serverUrl, search, ruleType, states, MAX_GROUPS),
    [serverUrl, search, ruleType, states],
  );

  const loaded = !!groups.length || !blockFetch;
  useEffect(() => {
    if (blockFetch) return;
    const fetchData = async () => {
      setIsLoading(true);
      try {
        const url = `${fetchUrl}&page_num=${pageNum}`;
        const response = await fetch(url);
        const resp = await response.json();
        if (response.ok) {
          const loadedGroups = (resp.data.groups || []) as Group[];
          setGroups(loadedGroups);
          setPageInfo({
            page: resp.page || 1,
            total_pages: resp.total_pages || 1,
            total_groups: resp.total_groups || 0,
            total_rules: resp.total_rules || 0,
          });
          setError(undefined);
        } else if (response.status === 400 && resp?.error.includes("exceeds total amount of pages")) {
          onPageChange(1)();
          setError(`${resp.errorType}\r\n${resp?.error}`);
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
  }, [fetchUrl, period, loaded, pageNum]);

  return { groups, isLoading, error, pageInfo };
};
