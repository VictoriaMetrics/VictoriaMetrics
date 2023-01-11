import { MetricBase } from "../api/types";

export const getNameForMetric = (result: MetricBase, alias?: string): string => {
  const { __name__, ...freeFormFields } = result.metric;
  const name = alias || `[Query ${result.group}] ${__name__ || ""}`;
  if (Object.keys(freeFormFields).length == 0) {
    return name;
  }
  return `${name}{${Object.entries(freeFormFields).map(e =>
    `${e[0]}=${JSON.stringify(e[1])}`
  ).join(", ")}}`;
};
