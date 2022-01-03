import {TimeParams} from "../types";

export const getQueryRangeUrl = (server: string, query: string, period: TimeParams, nocache: boolean): string =>
  `${server}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${period.start}&end=${period.end}&step=${period.step}${nocache ? "&nocache=1" : ""}`;

export const getQueryUrl = (server: string, query: string, period: TimeParams): string =>
  `${server}/api/v1/query?query=${encodeURIComponent(query)}&start=${period.start}&end=${period.end}&step=${period.step}`;

export const getQueryOptions = (server: string) => `${server}/api/v1/label/__name__/values`;
