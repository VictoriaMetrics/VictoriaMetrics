import qs from "qs";
import get from "lodash.get";
import router from "../router";

const graphStateToUrlParams = {
  "time.duration": "range_input",
  "time.period.date": "end_input",
  "time.period.step": "step_input",
  "time.relativeTime": "relative_time",
  "displayType": "tab",
};

const stateToUrlParams = {
  [router.home]: graphStateToUrlParams,
  [router.dashboards]: graphStateToUrlParams,
  [router.cardinality]: {
    "topN": "topN",
    "date": "date",
    "match": "match[]",
    "extraLabel": "extra_label",
    "focusLabel": "focusLabel"
  }
};

// TODO need function for detect types.
// const decoder = (value: string)  => {
// This function does not parse dates
//   if (/^(\d+|\d*\.\d+)$/.test(value)) {
//     return parseFloat(value);
//   }
//
//   const keywords = {
//     true: true,
//     false: false,
//     null: null,
//     undefined: undefined,
//   };
//   if (value in keywords) {
//     return keywords[value as keyof typeof keywords];
//   }
//
//   return decodeURI(value);
// };

export const setQueryStringWithoutPageReload = (qsValue: string): void => {
  const w = window;
  if (w) {
    const qs = qsValue ? `?${qsValue}` : "";
    const newurl = `${w.location.protocol}//${w.location.host}${w.location.pathname}${qs}${w.location.hash}`;
    w.history.pushState({ path: newurl }, "", newurl);
  }
};

export const setQueryStringValue = (newValue: Record<string, unknown>): void => {
  const route = window.location.hash.replace("#", "");
  const params = stateToUrlParams[route] || graphStateToUrlParams;
  const queryMap = new Map(Object.entries(params));
  const isGraphRoute = route === router.home || route === router.dashboards || !route;
  const newQsValue = isGraphRoute ? getGraphQsValue(newValue, queryMap) : getQsValue(newValue, queryMap);
  setQueryStringWithoutPageReload(newQsValue.join("&"));
};

const getGraphQsValue = (newValue: Record<string, unknown>, queryMap: Map<string, string>): string[] => {
  const query = get(newValue, "query", []) as string[];
  const newQsValue: string[] = [];
  query.forEach((q, i) => {
    queryMap.forEach((queryKey, stateKey) => {
      const value = get(newValue, stateKey, "") as string;
      if (value) {
        const valueEncoded = encodeURIComponent(value);
        newQsValue.push(`g${i}.${queryKey}=${valueEncoded}`);
      }
    });
    newQsValue.push(`g${i}.expr=${encodeURIComponent(q)}`);
  });

  return newQsValue;
};

const getQsValue = (newValue: Record<string, unknown>, queryMap: Map<string, string>): string[] => {
  const newQsValue: string[] = [];
  queryMap.forEach((queryKey, stateKey) => {
    const value = get(newValue, stateKey, "") as string;
    if (value) {
      const valueEncoded = encodeURIComponent(value);
      newQsValue.push(`${queryKey}=${valueEncoded}`);
    }
  });

  return newQsValue;
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
  return new Array(queryLength).fill(1).map((q, i) => {
    return getQueryStringValue(`g${i}.expr`, "") as string;
  });
};
