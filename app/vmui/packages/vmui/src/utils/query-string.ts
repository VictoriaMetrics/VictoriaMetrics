import qs from "qs";
import get from "lodash.get";

const stateToUrlParams = {
  "query": "g0.expr",
  "time.duration": "g0.range_input",
  "time.period.date": "g0.end_input",
  "time.period.step": "g0.step_input",
  "stacked": "g0.stacked",
};

// TODO need function for detect types. This function does not parse dates
// const decoder = (value: string)  => {
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
    const value = get(newValue, stateKey, "");
    if (value) {
      newQsValue.push(`${queryKey}=${value}`);
    }
  });
  setQueryStringWithoutPageReload(newQsValue.join("&"));
};

export const getQueryStringValue = (
  key: string,
  queryString = window.location.search
): string => {
  const values = qs.parse(queryString, { ignoreQueryPrefix: true});
  return String(get(values, key, ""));
};