import { InstantMetricResult } from "../../api/types";
import { getColumns, MetricCategory } from "../../hooks/useSortedCategories";
import { formatValueToCSV } from "../../utils/csv";

const getHeaders = (data: InstantMetricResult[]): string => {
  return getColumns(data).map(({ key }) => key).join(",");
};

const getRows = (data: InstantMetricResult[], headers: MetricCategory[]) => {
  return data?.map(d => headers.map(c => formatValueToCSV(d.metric[c.key] || "-")).join(","));
};

export const convertMetricsDataToCSV = (data: InstantMetricResult[]): string => {
  const headers = getHeaders(data);
  if (!headers.length) return "";
  const rows = getRows(data, getColumns(data));
  return [headers, ...rows].join("\n");
};
