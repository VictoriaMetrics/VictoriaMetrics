import qs from "qs";
import get from "lodash.get";
import { MAX_QUERY_FIELDS } from "../constants/graph";

export const getQueryStringValue = (
  key: string,
  defaultValue?: unknown,
): unknown => {
  const queryString = window.location.hash.split("?")[1];
  const values = qs.parse(queryString, { ignoreQueryPrefix: true });
  return get(values, key, defaultValue || "");
};

export const getQueryArray = (): string[] => {
  const queryString = window.location.hash.split("?")[1] || "";
  const queryLength = queryString.match(/g\d+\.expr/g)?.length || 1;
  return new Array(queryLength > MAX_QUERY_FIELDS ? MAX_QUERY_FIELDS : queryLength)
    .fill(1)
    .map((q, i) => getQueryStringValue(`g${i}.expr`, "") as string);
};
