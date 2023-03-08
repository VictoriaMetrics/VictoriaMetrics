import { Containers, Tabs, TSDBStatus } from "./types";
import { useRef } from "preact/compat";
import { HeadCell } from "./Table/types";

interface AppState {
  tabs: Tabs;
  containerRefs: Containers<HTMLDivElement>;
}

export default class AppConfigurator {
  private tsdbStatus: TSDBStatus;
  private tabsNames: string[];

  constructor() {
    this.tsdbStatus = this.defaultTSDBStatus;
    this.tabsNames = ["table", "graph"];
    this.getDefaultState = this.getDefaultState.bind(this);
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
      totalSeriesByAll: 0,
      seriesCountByMetricName: [],
      seriesCountByLabelName: [],
      seriesCountByFocusLabelValue: [],
      seriesCountByLabelValuePair: [],
      labelValueCountByLabelName: [],
    };
  }

  keys(match?: string | null, focusLabel?: string | null): string[] {
    const isMetric = match && match.includes("__name__");
    let keys: string[] = [];
    if (focusLabel) {
      keys = keys.concat("seriesCountByFocusLabelValue");

    } else if (isMetric) {
      keys = keys.concat("labelValueCountByLabelName", "seriesCountByLabelValuePair");
    } else {
      keys = keys.concat("seriesCountByMetricName", "seriesCountByLabelName",);
    }
    return keys;
  }

  getDefaultState(match?: string | null, label?: string | null): AppState {
    return this.keys(match, label).reduce((acc, cur) => {
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
      };
    }, {
      tabs: {} as Tabs,
      containerRefs: {} as Containers<HTMLDivElement>,
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

  get sectionsTips(): Record<string, string> {
    return {
      seriesCountByMetricName: `
        <p>
          This table returns a list of the highest cardinality metrics in the selected data source. 
          The cardinality of a metric is the number of time series associated with that metric, 
          where each time series is defined as a unique combination of key-value label pairs.
        </p>
        <p>
          When looking to reduce the number of active series in your data source, 
          you can start by inspecting individual metrics with high cardinality
          (i.e. that have lots of active time series associated with them), 
          since that single metric contributes a large fraction of the series that make up your total series count.
        </p>`,
      seriesCountByLabelName: `
        <p>
          This table returns a list of the label keys with the highest number of values.
        </p>
        <p>
          Use this table to identify labels that are storing dimensions with high cardinality 
          (many different label values), such as user IDs, email addresses, or other unbounded sets of values.
        </p> 
        <p>
          We advise being careful in choosing labels such that they have a finite set of values, 
          since every unique combination of key-value label pairs creates a new time series 
          and therefore can dramatically increase the number of time series in your system.
        </p>`,
      seriesCountByFocusLabelValue: "",
      labelValueCountByLabelName: "",
      seriesCountByLabelValuePair: "",
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
    id: "name",
    label: "Metric name",
  },
  {
    id: "value",
    label: "Number of series",
  },
  {
    id: "percentage",
    label: "Percent of series",
  },
  {
    id: "action",
    label: "",
  }
] as HeadCell[];

const LABEL_NAMES_HEADERS = [
  {
    id: "name",
    label: "Label name",
  },
  {
    id: "value",
    label: "Number of series",
  },
  {
    id: "percentage",
    label: "Percent of series",
  },
  {
    id: "action",
    label: "",
  }
] as HeadCell[];

const FOCUS_LABEL_VALUES_HEADERS = [
  {
    id: "name",
    label: "Label value",
  },
  {
    id: "value",
    label: "Number of series",
  },
  {
    id: "percentage",
    label: "Percent of series",
  },
  {
    disablePadding: false,
    id: "action",
    label: "",
    numeric: false,
  }
] as HeadCell[];

export const LABEL_VALUE_PAIRS_HEADERS = [
  {
    id: "name",
    label: "Label=value pair",
  },
  {
    id: "value",
    label: "Number of series",
  },
  {
    id: "percentage",
    label: "Percent of series",
  },
  {
    id: "action",
    label: "",
  }
] as HeadCell[];

export const LABEL_NAMES_WITH_UNIQUE_VALUES_HEADERS = [
  {
    id: "name",
    label: "Label name",
  },
  {
    id: "value",
    label: "Number of unique values",
  },
  {
    id: "action",
    label: "",
  }
] as HeadCell[];
