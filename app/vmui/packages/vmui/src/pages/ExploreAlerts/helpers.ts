import { Rule } from "../../types";

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

export const getFromStorage = (key: string) => {
  const stateDump = window.localStorage.getItem(key) || "";
  return new Set<string>(stateDump.split(",").filter(s => s));
};
