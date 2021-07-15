import qs from "qs";

const decoder = (value: string)  => {
  if (/^(\d+|\d*\.\d+)$/.test(value)) {
    return parseFloat(value);
  }

  const keywords = {
    true: true,
    false: false,
    null: null,
    undefined: undefined,
  };
  if (value in keywords) {
    return keywords[value as keyof typeof keywords];
  }

  return decodeURI(value);
};

export const setQueryStringWithoutPageReload = (qsValue: string): void => {
  const w = window;
  if (w) {
    const newurl = w.location.protocol +
        "//" +
        w.location.host +
        w.location.pathname +
        "?" +
        qsValue;
    w.history.pushState({ path: newurl }, "", newurl);
  }
};

export const setQueryStringValue = (
  newValue: Record<string, unknown>,
  queryString = window.location.search
): void => {
  const values = qs.parse(queryString, { ignoreQueryPrefix: true, decoder });
  const newQsValue = qs.stringify({ ...values, ...newValue }, { encode: false });
  setQueryStringWithoutPageReload(newQsValue);
};

export const getQueryStringValue = (
  key: string,
  queryString = window.location.search
): unknown => {
  const values = qs.parse(queryString, { ignoreQueryPrefix: true, decoder });
  return values[key];
};