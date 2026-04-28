import { TimeParams } from "../types";

export const getQueryRangeUrl = (server: string, query: string, period: TimeParams, nocache: boolean, queryTracing: boolean): string =>
  `${server}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${period.start}&end=${period.end}&step=${period.step}${nocache ? "&nocache=1" : ""}${queryTracing ? "&trace=1" : ""}`;

export const getQueryUrl = (server: string, query: string, period: TimeParams, nocache: boolean, queryTracing: boolean): string =>
  `${server}/api/v1/query?query=${encodeURIComponent(query)}&time=${period.end}&step=${period.step}${nocache ? "&nocache=1" : ""}${queryTracing ? "&trace=1" : ""}`;

export const getExportDataUrl = (server: string, query: string, period: TimeParams, reduceMemUsage: boolean): string => {
  const params = new URLSearchParams({
    "match[]": query,
    start: period.start.toString(),
    end: period.end.toString(),
  });
  if (reduceMemUsage) params.set("reduce_mem_usage", "1");
  return `${server}/api/v1/export?${params}`;
};

const getBaseParams = (period: TimeParams, query: string[]): URLSearchParams => {
  const params = new URLSearchParams({
    start: period.start.toString(),
    end: period.end.toString(),
  });
  query.forEach((q => params.append("match[]", q)));
  return params;
};

export const getLabelsUrl = (server: string, query: string[], period: TimeParams): string => {
  const params = getBaseParams(period, query);
  return `${server}/api/v1/labels?${params}`;
};

export const getExportCSVDataUrl = (server: string, query: string[], period: TimeParams, reduceMemUsage: boolean, format: string): string => {
  const params = getBaseParams(period, query);
  params.set("format", format);
  if (reduceMemUsage) params.set("reduce_mem_usage", "1");
  return `${server}/api/v1/export/csv?${params}`;
};

export const getExportJSONDataUrl = (server: string, query: string[], period: TimeParams, reduceMemUsage: boolean): string => {
  const params = getBaseParams(period, query);
  if (reduceMemUsage) params.set("reduce_mem_usage", "1");
  return `${server}/api/v1/export?${params}`;
};
