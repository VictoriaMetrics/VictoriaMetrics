import { InstantMetricResult } from "../../api/types";
import { getColumns, MetricCategory } from "../../hooks/useSortedCategories";
import { formatValueToCSV } from "../../utils/csv";

const getHeaders = (data: InstantMetricResult[]): string => {
  const metricHeaders = getColumns(data).map(({ key }) => key).join(",");
  if (!metricHeaders) {
    return metricHeaders;
  }
  return `${metricHeaders},__timestamp__,__value__`;
};

const getRows = (data: InstantMetricResult[], headers: MetricCategory[]) => {
  return data?.map(d => {
    const metricPart = headers.map(c => formatValueToCSV(d.metric[c.key] || "-")).join(",");
    const timestamp = d.value ? formatValueToCSV(String(d.value[0])) : "-";
    const value = d.value ? formatValueToCSV(d.value[1]) : "-";
    return `${metricPart},${timestamp},${value}`;
  });
};

export const convertMetricsDataToCSV = (data: InstantMetricResult[]): string => {
  const headers = getHeaders(data);
  if (!headers.length) return "";
  const rows = getRows(data, getColumns(data));
  return [headers, ...rows].join("\n");
};
