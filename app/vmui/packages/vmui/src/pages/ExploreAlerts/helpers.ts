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
  if (!rule?.lastSamples && !rule?.lastSeriesFetched) {
    state = "no match";
  } else if (rule?.health != "ok") {
    state = "unhealthy";
  }
  return state;
};

export const getFromStorage = (key: string) => {
  const stateDump = window.localStorage.getItem(key) || "";
  return new Set<string>(stateDump.split(",").filter(s => s));
};
