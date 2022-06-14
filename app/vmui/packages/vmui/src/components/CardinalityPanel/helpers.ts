import {Containers, DefaultState, QueryUpdater, Tabs, TSDBStatus} from "./types";
import {useRef} from "preact/compat";

export const queryUpdater: QueryUpdater = {
  labelValueCountByLabelName: (query: string): string => `{${query}!=""}`,
  seriesCountByLabelValuePair: (query: string): string => {
    const a = query.split("=");
    const label = a[0];
    const value = a.slice(1).join("=");
    return getSeriesSelector(label, value);
  },
  seriesCountByMetricName: (query: string): string => {
    return getSeriesSelector("__name__", query);
  },
};

const getSeriesSelector = (label: string, value: string): string => {
  return "{" + label + "=" + JSON.stringify(value) + "}";
};

export const defaultProperties = (tsdbStatus: TSDBStatus) => {
  return Object.keys(tsdbStatus).reduce((acc, key) => {
    if (key === "totalSeries" || key === "totalLabelValuePairs") return acc;
    return {
      ...acc,
      tabs:{
        ...acc.tabs,
        [key]: ["table", "graph"],
      },
      containerRefs: {
        ...acc.containerRefs,
        [key]: useRef<HTMLDivElement>(null),
      },
      defaultState: {
        ...acc.defaultState,
        [key]: 0,
      },
    };
  }, {
    tabs:{} as Tabs,
    containerRefs: {} as Containers<HTMLDivElement>,
    defaultState: {} as DefaultState,
  });
};
