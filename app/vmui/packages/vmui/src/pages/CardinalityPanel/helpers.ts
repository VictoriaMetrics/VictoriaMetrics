import { QueryUpdater } from "./types";

export const queryUpdater: QueryUpdater = {
  seriesCountByMetricName: ({ query }): string => {
    return getSeriesSelector("__name__", query);
  },
  seriesCountByLabelName: ({ query }): string => {
    return `{${query}!=""}`;
  },
  seriesCountByFocusLabelValue: ({ query, focusLabel }): string => {
    return getSeriesSelector(focusLabel, query);
  },
  seriesCountByLabelValuePair: ({ query }): string => {
    const a = query.split("=");
    const label = a[0];
    const value = a.slice(1).join("=");
    return getSeriesSelector(label, value);
  },
  labelValueCountByLabelName: ({ query, match }): string => {
    if (match === "") {
      return `{${query}!=""}`;
    }
    return `${match.replace("}", "")}, ${query}!=""}`;
  },
};

const getSeriesSelector = (label: string | null, value: string): string => {
  if (!label) {
    return "";
  }
  return "{" + label + "=" + JSON.stringify(value) + "}";
};
