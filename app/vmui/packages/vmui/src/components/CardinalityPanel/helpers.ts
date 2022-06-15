import {QueryUpdater} from "./types";

export const queryUpdater: QueryUpdater = {
  seriesCountByMetricName: (query: string): string => {
    return getSeriesSelector("__name__", query);
  },
  seriesCountByLabelName: (query: string): string => `{${query}!=""}`,
  seriesCountByLabelValuePair: (query: string): string => {
    const a = query.split("=");
    const label = a[0];
    const value = a.slice(1).join("=");
    return getSeriesSelector(label, value);
  },
  labelValueCountByLabelName: (query: string): string => `{${query}!=""}`,
};

const getSeriesSelector = (label: string, value: string): string => {
  return "{" + label + "=" + JSON.stringify(value) + "}";
};
