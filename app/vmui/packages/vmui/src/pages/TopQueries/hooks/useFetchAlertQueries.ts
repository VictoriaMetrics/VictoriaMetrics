import { useEffect, useMemo, useState } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import { Group } from "../../../types";

const getAllRulesUrl = (server: string): string =>
  `${server}/vmalert/api/v1/rules?datasource_type=prometheus&group_limit=1000`;

type AlertRuleRef = { group_id: string; rule_id: string };

export const useFetchAlertQueries = (): Map<string, AlertRuleRef> => {
  const { serverUrl, appConfig } = useAppState();
  const [alertQueries, setAlertQueries] = useState<Map<string, AlertRuleRef>>(new Map());

  const isEnabled = appConfig?.vmalert?.enabled ?? false;

  const fetchUrl = useMemo(
    () => (isEnabled ? getAllRulesUrl(serverUrl) : null),
    [serverUrl, isEnabled],
  );

  useEffect(() => {
    if (!fetchUrl) return;
    const fetchData = async () => {
      try {
        const response = await fetch(fetchUrl);
        if (!response.ok) return;
        const resp = await response.json();
        const groups = (resp?.data?.groups || []) as Group[];
        const queries = new Map<string, AlertRuleRef>();
        for (const group of groups) {
          for (const rule of group.rules) {
            if (rule.query && !queries.has(rule.query)) {
              queries.set(rule.query, { group_id: group.id, rule_id: rule.id });
            }
          }
        }
        setAlertQueries(queries);
      } catch (e) {
        // silently ignore fetch errors
      }
    };
    fetchData();
  }, [fetchUrl]);

  return alertQueries;
};
