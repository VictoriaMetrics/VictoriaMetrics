import { useMemo, useCallback, useEffect, useState } from "preact/compat";
import { getGroupsUrl } from "../../../api/explore-alerts";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes, Group } from "../../../types";
import { useTimeState } from "../../../state/time/TimeStateContext";

interface FetchGroupsReturn {
  groups: Group[];
  isLoading: boolean;
  error?: ErrorTypes | string;
  hasMoreNext: boolean;
  hasMoreBefore: boolean;
  loadGroups: () => void;
}

interface FetchGroupsProps {
  blockFetch: boolean;
  search: string;
  ruleType: string;
  states: string[];
  startToken: string;
}

const MAX_GROUPS = 100;

export const useFetchGroups = ({ blockFetch, startToken, search, ruleType, states }: FetchGroupsProps): FetchGroupsReturn => {
  const { serverUrl } = useAppState();
  const { period } = useTimeState();

  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [hasMoreNext, setHasMoreNext] = useState(true);
  const [hasMoreBefore, setHasMoreBefore] = useState(!!startToken);
  const [groupNextToken, setGroupNextToken] = useState(startToken || "");
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchUrl = useMemo(
    () => getGroupsUrl(serverUrl, search, ruleType, states, MAX_GROUPS),
    [serverUrl, search, ruleType, states],
  );

  const loadGroups = useCallback(async(nextToken = groupNextToken) => {
    if (blockFetch) return;
    setIsLoading(true);
    let newGroups = groups;
    if (nextToken !== groupNextToken || !nextToken) {
      newGroups = [];
      setHasMoreBefore(!!nextToken);
    }
    try {
      const url = `${fetchUrl}&group_next_token=${nextToken}`;
      const response = await fetch(url);
      const resp = await response.json();
      if (response.ok) {
        const loadedGroups = (resp.data.groups || []) as Group[];
        setGroups([...newGroups, ...loadedGroups]);
        setHasMoreNext(!!resp.data.groupNextToken);
        setGroupNextToken(resp.data.groupNextToken || "");
        setError(undefined);
      } else if (response.status === 400) {
        setGroups([]);
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
  }, [fetchUrl, blockFetch, groupNextToken]);

  useEffect(() => {
    loadGroups(startToken || "");
  }, [period, fetchUrl, startToken]);

  return { groups, isLoading, error, hasMoreNext, hasMoreBefore, loadGroups };
};
