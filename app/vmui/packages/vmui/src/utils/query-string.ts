import qs from "qs";
import get from "lodash.get";
import { MAX_QUERY_FIELDS } from "../constants/config";

export const setQueryStringWithoutPageReload = (params: Record<string, unknown>): void => {
  const w = window;
  if (w) {
    const qsValue = Object.entries(params).map(([k, v]) => `${k}=${v}`).join("&");
    const qs = qsValue ? `?${qsValue}` : "";
    const newurl = `${w.location.protocol}//${w.location.host}${w.location.pathname}${qs}${w.location.hash}`;
    w.history.pushState({ path: newurl }, "", newurl);
  }
};

export const getQueryStringValue = (
  key: string,
  defaultValue?: unknown,
  queryString = window.location.search
): unknown => {
  const values = qs.parse(queryString, { ignoreQueryPrefix: true });
  return get(values, key, defaultValue || "");
};

export const getQueryArray = (): string[] => {
  const queryLength = window.location.search.match(/g\d+.expr/gmi)?.length || 1;
  return new Array(queryLength > MAX_QUERY_FIELDS ? MAX_QUERY_FIELDS : queryLength)
    .fill(1)
    .map((q, i) => getQueryStringValue(`g${i}.expr`, "") as string);
};
