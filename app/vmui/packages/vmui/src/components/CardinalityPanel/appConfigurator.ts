import {Containers, DefaultActiveTab, Tabs, TSDBStatus} from "./types";
import {useRef} from "preact/compat";
import {HeadCell} from "../Table/types";

interface AppState {
  tabs: Tabs;
  containerRefs: Containers<HTMLDivElement>;
  defaultActiveTab: DefaultActiveTab,
}

export default class AppConfigurator {
  private tsdbStatus: TSDBStatus;
  private tabsNames: string[];

  constructor() {
    this.tsdbStatus = this.defaultTSDBStatus;
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
      seriesCountByMetricName: [],
      seriesCountByLabelName: [],
      seriesCountByFocusLabelValue: [],
      seriesCountByLabelValuePair: [],
      labelValueCountByLabelName: [],
    };
  }

  keys(focusLabel: string | null): string[] {
    let keys: string[] = [];
    if (focusLabel) {
      keys = keys.concat("seriesCountByFocusLabelValue");
    }
    keys = keys.concat(
      "seriesCountByMetricName",
      "seriesCountByLabelName",
      "seriesCountByLabelValuePair",
      "labelValueCountByLabelName",
    );
    return keys;
  }

  get defaultState(): AppState {
    return this.keys("job").reduce((acc, cur) => {
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
      seriesCountByFocusLabelValue: `Values for "${str}" label with the highest number of series`,
      seriesCountByLabelValuePair: "Label=value pairs with the highest number of series",
      labelValueCountByLabelName: "Labels with the highest number of unique values",
    };
  }

  get tablesHeaders(): Record<string, HeadCell[]> {
    return {
      seriesCountByMetricName: METRIC_NAMES_HEADERS,
      seriesCountByLabelName: LABEL_NAMES_HEADERS,
      seriesCountByFocusLabelValue: FOCUS_LABEL_VALUES_HEADERS,
      seriesCountByLabelValuePair: LABEL_VALUE_PAIRS_HEADERS,
      labelValueCountByLabelName: LABEL_NAMES_WITH_UNIQUE_VALUES_HEADERS,
    };
  }

  totalSeries(keyName: string): number {
    if (keyName === "labelValueCountByLabelName") {
      return -1;
    }
    return this.tsdbStatus.totalSeries;
  }
}

const METRIC_NAMES_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Metric name",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Number of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "percentage",
    label: "Percent of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];

const LABEL_NAMES_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Label name",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Number of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "percentage",
    label: "Percent of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];

const FOCUS_LABEL_VALUES_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Label value",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Number of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "percentage",
    label: "Percent of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];

export const LABEL_VALUE_PAIRS_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Label=value pair",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Number of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "percentage",
    label: "Percent of series",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];

export const LABEL_NAMES_WITH_UNIQUE_VALUES_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Label name",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "value",
    label: "Number of unique values",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
] as HeadCell[];
