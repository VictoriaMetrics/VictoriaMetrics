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
  private isPrometheus: boolean;

  constructor() {
    this.tsdbStatus = this.defaultTSDBStatus;
    this.tabsNames = ["table", "graph"];
    this.isPrometheus = false;
    this.getDefaultState = this.getDefaultState.bind(this);
  }

  set tsdbStatusData(tsdbStatus: TSDBStatus) {
    this.isPrometheus = !!tsdbStatus?.headStats;
    this.tsdbStatus = tsdbStatus;
  }

  get tsdbStatusData(): TSDBStatus {
    return this.tsdbStatus;
  }

  get defaultTSDBStatus(): TSDBStatus {
    return {
      totalSeries: 0,
      totalSeriesPrev: 0,
      totalSeriesByAll: 0,
      totalLabelValuePairs: 0,
      seriesCountByMetricName: [],
      seriesCountByLabelName: [],
      seriesCountByFocusLabelValue: [],
      seriesCountByLabelValuePair: [],
      labelValueCountByLabelName: [],
    };
  }

  get isPrometheusData(): boolean {
    return this.isPrometheus;
  }

  keys(match?: string | null, focusLabel?: string | null): string[] {
    const isMetric = match && /__name__=".+"/.test(match);
    const isLabel = match && /{.+=".+"}/g.test(match);
    const isMetricWithLabel = match && /__name__=".+", .+!=""/g.test(match);

    let keys: string[] = [];
    if (focusLabel || isMetricWithLabel) {
      keys = keys.concat("seriesCountByFocusLabelValue");
    } else if (isMetric) {
      keys = keys.concat("labelValueCountByLabelName");
    } else if (isLabel) {
      keys = keys.concat("seriesCountByMetricName", "seriesCountByLabelName");
    } else {
      keys = keys.concat("seriesCountByMetricName", "seriesCountByLabelName", "seriesCountByLabelValuePair", "labelValueCountByLabelName");
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
          This table returns a list of metrics with the highest cardinality.
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
          This table returns a list of the labels with the highest number of series.
        </p>
        <p>
          Use this table to identify labels that are storing dimensions with high cardinality
          (many different label values).
        </p>
        <p>
          It is recommended to choose labels such that they have a finite set of values,
          since every unique combination of key-value label pairs creates a new time series
          and therefore can dramatically increase the number of time series in your system.
        </p>`,
      seriesCountByFocusLabelValue: `
       <p>
          This table returns a list of unique label values per selected label.
       </p>
       <p>
          Use this table to identify label values that are storing per each selected series.
       </p>`,
      labelValueCountByLabelName: `
       <p>
          This table returns a list of labels with the highest number of the unique values.
       </p>
      `,
      seriesCountByLabelValuePair: `
        <p>
          This table returns a list of the label values pairs with the highest number of series.
        </p>
        <p>
          Use this table to identify unique label values pairs. This helps to identify same labels 
          is applied to count timeseries in your system, since every unique combination of key-value label pairs 
          creates a new time series and therefore can dramatically increase the number of time series in your system
        </p>`,
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

  totalSeries(keyName: string, prev = false): number {
    if (keyName === "labelValueCountByLabelName") {
      return -1;
    }
    return prev ? this.tsdbStatus.totalSeriesPrev : this.tsdbStatus.totalSeries;
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
    label: "Share in total",
    info: "Shows the share of a metric to the total number of series"
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
    label: "Share in total",
    info: "Shows the share of the label to the total number of series"
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
    label: "Share in total",
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
    label: "Share in total",
    info: "Shows the share of the label value pair to the total number of series"
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
