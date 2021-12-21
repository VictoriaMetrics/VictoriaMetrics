import {MetricBase} from "../api/types";

export const getNameForMetric = (result: MetricBase): string => {
  if (Object.keys(result.metric).length === 0) {
    return `Query ${result.group} result`; // a bit better than just {} for case of aggregation functions
  }
  const { __name__: name, ...freeFormFields } = result.metric;
  return `${name || ""} {${Object.entries(freeFormFields).map(e => `${e[0]}: ${e[1]}`).join(", ")}}`;
};
