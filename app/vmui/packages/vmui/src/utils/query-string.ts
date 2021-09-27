import qs from "qs";
import get from "lodash.get";

const stateToUrlParams = {
  "query": "g0.expr",
  "time.duration": "g0.range_input",
  "time.period.date": "g0.end_input",
  "time.period.step": "g0.step_input",
  "stacked": "g0.stacked",
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
    const newurl = `${w.location.protocol}//${w.location.host}${w.location.pathname}?${qsValue}`;
    w.history.pushState({ path: newurl }, "", newurl);
  }
};

export const setQueryStringValue = (newValue: Record<string, unknown>): void => {
  const queryMap = new Map(Object.entries(stateToUrlParams));
  const newQsValue: string[] = [];
  queryMap.forEach((queryKey, stateKey) => {
    const queryKeyEncoded = encodeURIComponent(queryKey);
    const value = get(newValue, stateKey, "") as string;
    if (value) {
      const valueEncoded = encodeURIComponent(value);
      newQsValue.push(`${queryKey}=${valueEncoded}`);
    }
  });
  setQueryStringWithoutPageReload(newQsValue.join("&"));
};

export const getQueryStringValue = (
  key: string,
  defaultValue?: unknown,
  queryString = window.location.search
): unknown => {
  const values = qs.parse(queryString, { ignoreQueryPrefix: true });
  return get(values, key, defaultValue || "");
};
