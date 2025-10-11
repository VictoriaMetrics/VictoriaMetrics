import { Rule, Group } from "../../types";

export const getChanges = (title: string, prevValues: string[]): string[] => {
  if (title === "All") return [];

  const newValues = new Set<string>(prevValues);
  if (newValues.has(title)) {
    newValues.delete(title);
  } else {
    newValues.add(title);
  }

  return Array.from(newValues);
};

export const getState = (rule: Rule) => {
  let state = rule?.state || "ok";
  if (rule?.health !== "ok") {
    state = "unhealthy";
  } else if (!rule?.lastSamples && !rule?.lastSeriesFetched) {
    state = "no match";
  }
  return state;
};

export const getStates = (rule: Rule) => {
  const output: Record<string, number> = {};
  const alertsCount = rule?.alerts?.length || 0;
  if (alertsCount > 0) {
    rule.alerts.forEach((alert) => {
      if (alert.state in output) {
        output[alert.state] += 1;
      } else {
        output[alert.state] = 1;
      }
    });
  } else {
    output[getState(rule)] = 1;
  }
  return output;
};

export const filterGroups = (groups: Group[], types: string[], states: string[], searchInput: string) => {
  const allTypes: Set<string> = new Set();
  const allStates: Set<string> = new Set();
  const filteredGroups: Group[] = [];

  groups.forEach((group) => {
    const filteredRules: Rule[] = [];
    const statesPerGroup: Record<string, number> = {};
    group.rules.forEach((rule) => {
      const ruleType = rule.type.charAt(0).toUpperCase() + rule.type.slice(1);
      allTypes.add(ruleType);
      if (types?.length && !types.includes(ruleType)) return;

      const state = getState(rule);
      const stateName = state.charAt(0).toUpperCase() + state.slice(1);
      allStates.add(stateName);
      if (states?.length && !states.includes(stateName)) return;

      if (
        searchInput &&
        !rule.name.toLowerCase().includes(searchInput.toLowerCase()) &&
        !group.name.toLowerCase().includes(searchInput.toLowerCase()) &&
        !group.file.toLowerCase().includes(searchInput.toLowerCase())
      )
        return;

      filteredRules.push(rule);
      if (state !== "no match" && state !== "unhealthy" && state !== "firing" && state !== "pending")
        return;

      const count = state === "firing" || state === "pending" ? rule?.alerts?.length : 1;
      if (stateName in statesPerGroup) {
        statesPerGroup[stateName] += count;
      } else {
        statesPerGroup[stateName] = count;
      }
    });
    if (filteredRules.length) {
      const g = Object.assign({}, group);
      g.rules = filteredRules;
      g.states = statesPerGroup;
      filteredGroups.push(g);
    }
  });
  return { filteredGroups, allTypes, allStates };
};
