import { TimeParams } from "../types";

export const getQueryRangeUrl = (server: string, query: string, period: TimeParams, nocache: boolean, queryTracing: boolean): string =>
  `${server}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${period.start}&end=${period.end}&step=${period.step}${nocache ? "&nocache=1" : ""}${queryTracing ? "&trace=1" : ""}`;

export const getQueryUrl = (server: string, query: string, period: TimeParams, nocache: boolean, queryTracing: boolean): string =>
  `${server}/api/v1/query?query=${encodeURIComponent(query)}&time=${period.end}&step=${period.step}${nocache ? "&nocache=1" : ""}${queryTracing ? "&trace=1" : ""}`;
