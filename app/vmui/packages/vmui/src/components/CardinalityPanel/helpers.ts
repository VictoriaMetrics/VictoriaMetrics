import {QueryUpdater} from "./types";

export const queryUpdater: QueryUpdater = {
  seriesCountByMetricName: (focusLabel: string | null, query: string): string => {
    return getSeriesSelector("__name__", query);
  },
  seriesCountByLabelName: (focusLabel: string | null, query: string): string => `{${query}!=""}`,
  seriesCountByFocusLabelValue: (focusLabel: string | null, query: string): string => {
    return getSeriesSelector(focusLabel, query);
  },
  seriesCountByLabelValuePair: (focusLabel: string | null, query: string): string => {
    const a = query.split("=");
    const label = a[0];
    const value = a.slice(1).join("=");
    return getSeriesSelector(label, value);
  },
  labelValueCountByLabelName: (focusLabel: string | null, query: string): string => `{${query}!=""}`,
};

const getSeriesSelector = (label: string | null, value: string): string => {
  if (!label) {
    return "";
  }
  return "{" + label + "=" + JSON.stringify(value) + "}";
};
