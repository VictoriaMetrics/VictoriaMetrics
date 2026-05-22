import { InstantMetricResult } from "../../api/types";
import { getColumns, MetricCategory } from "../../hooks/useSortedCategories";
import { formatValueToCSV } from "../../utils/csv";

const getHeaders = (data: InstantMetricResult[]): string => {
  const metricHeaders = getColumns(data).map(({ key }) => key);
  return [...metricHeaders, "__timestamp__", "__value__"].join(",");
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
  if (!data.length) return "";
  const headers = getHeaders(data);
  const rows = getRows(data, getColumns(data));
  return [headers, ...rows].join("\n");
};
