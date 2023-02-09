import { MetricBase } from "../api/types";

export const getNameForMetric = (result: MetricBase, alias?: string, showQueryNum = true): string => {
  const { __name__, ...freeFormFields } = result.metric;
  const name = alias || `${showQueryNum ? `[Query ${result.group}] ` : ""}${__name__ || ""}`;
  if (Object.keys(freeFormFields).length == 0) {
    if (!name) {
      return "value";
    }
    return name;
  }
  return `${name}{${Object.entries(freeFormFields).map(e =>
    `${e[0]}=${JSON.stringify(e[1])}`
  ).join(", ")}}`;
};

export const promValueToNumber = (s: string): number => {
  // See https://prometheus.io/docs/prometheus/latest/querying/api/#expression-query-result-formats
  switch (s) {
    case "NaN":
      return NaN;
    case "Inf":
    case "+Inf":
      return Infinity;
    case "-Inf":
      return -Infinity;
    default:
      return parseFloat(s);
  }
};
