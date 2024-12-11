import { MetricBase } from "../api/types";

export const getNameForMetric = (result: MetricBase, alias?: string, showQueryNum = true): string => {
  const { __name__, ...freeFormFields } = result.metric;
  const queryPrefix = showQueryNum ? `[Query ${result.group}] ` : "";

  if (alias) {
    return alias.replace(/\{\{(\w+)}}/g, (_, key) => result.metric[key] || "");
  }

  const name = `${queryPrefix}${__name__ || ""}`;

  if (Object.keys(freeFormFields).length === 0) {
    return name || "value";
  }

  const fieldsString = Object.entries(freeFormFields)
    .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
    .join(", ");

  return `${name}{${fieldsString}}`;
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

export const isHistogramData = (result: MetricBase[]) => {
  if (result.length < 2) return false;
  const histogramLabels = ["le", "vmrange"];

  const firstLabels = Object.keys(result[0].metric).filter(n => !histogramLabels.includes(n));
  const isHistogram = result.every(r => {
    const labels = Object.keys(r.metric).filter(n => !histogramLabels.includes(n));
    return firstLabels.length === labels.length && labels.every(l => r.metric[l] === result[0].metric[l]);
  });

  return isHistogram && result.every(r => histogramLabels.some(l => l in r.metric));
};
