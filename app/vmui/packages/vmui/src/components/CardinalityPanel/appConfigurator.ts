import {Containers, DefaultActiveTab, Tabs, TSDBStatus} from "./types";
import {useRef} from "preact/compat";
import {
  LABEL_NAMES_HEADERS,
  LABEL_VALUE_PAIRS_HEADERS, LABEL_WITH_HIGHEST_SERIES_HEADERS,
  LABELS_WITH_UNIQUE_VALUES_HEADERS,
  METRIC_NAMES_HEADERS
} from "./consts";
import {HeadCell} from "../Table/types";

interface AppState {
  tabs: Tabs;
  containerRefs: Containers<HTMLDivElement>;
  defaultActiveTab: DefaultActiveTab,
}

export default class AppConfigurator {
  private tsdbStatus: TSDBStatus;
  private totalFields: string[];
  private tabsNames: string[];

  constructor() {
    this.tsdbStatus = this.defaultTSDBStatus;
    this.totalFields = ["totalSeries", "totalLabelValuePairs"];
    this.tabsNames = ["table", "graph"];
  }

  set tsdbStatusData(tsdbStatus: TSDBStatus) {
    this.tsdbStatus = tsdbStatus;
  }

  get tsdbStatusData(): TSDBStatus {
    return this.tsdbStatus;
  }

  get defaultTSDBStatus(): TSDBStatus {
    return {
      totalSeries: 0,
      totalLabelValuePairs: 0,
      seriesCountByFocusLabelValue: [],
      seriesCountByMetricName: [],
      seriesCountByLabelName: [],
      seriesCountByLabelValuePair: [],
      labelValueCountByLabelName: [],
    };
  }

  get keys(): string[] {
    return Object.keys(this.tsdbStatus);
  }

  get keysWithoutTotalFields(): string[] {
    return this.keys.filter((keyName: string) => this.totalFields.indexOf(keyName) === -1);
  }

  get defaultState(): AppState {
    return this.keysWithoutTotalFields.reduce((acc, cur) => {
      return {
        ...acc,
        tabs: {
          ...acc.tabs,
          [cur]: this.tabsNames,
        },
        containerRefs: {
          ...acc.containerRefs,
          [cur]: useRef<HTMLDivElement>(null),
        },
        defaultActiveTab: {
          ...acc.defaultActiveTab,
          [cur]: 0,
        },
      };
    }, {
      tabs: {} as Tabs,
      containerRefs: {} as Containers<HTMLDivElement>,
      defaultActiveTab: {} as DefaultActiveTab,
    } as AppState);
  }

  sectionsTitles(str: string | null): Record<string, string> {
    return {
      seriesCountByMetricName: "Metric names with the highest number of series",
      seriesCountByLabelName: "Labels with the highest number of series",
      seriesCountByLabelValuePair: "Label=value pairs with the highest number of series",
      labelValueCountByLabelName: "Labels with the highest number of unique values",
      seriesCountByFocusLabelValue: `{${str}} label values with the highest number of series`,
    };
  }

  get tablesHeaders(): Record<string, HeadCell[]> {
    return {
      seriesCountByFocusLabelValue: LABEL_WITH_HIGHEST_SERIES_HEADERS,
      seriesCountByMetricName: METRIC_NAMES_HEADERS,
      seriesCountByLabelName: LABEL_NAMES_HEADERS,
      seriesCountByLabelValuePair: LABEL_VALUE_PAIRS_HEADERS,
      labelValueCountByLabelName: LABELS_WITH_UNIQUE_VALUES_HEADERS,
    };
  }

  totalSeries(keyName: string): number {
    if (keyName === "labelValueCountByLabelName" || keyName === "seriesCountByFocusLabelValue") {
      return -1;
    }
    return this.tsdbStatus.totalSeries;
  }
}
