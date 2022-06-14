import {HeadCell} from "../Table/types";

export const METRICS_TABLE_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Metrics name",
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
]as HeadCell[];

export const LABEL_VALUE_PAIRS_TABLE_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Lable=value pair",
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
    label: "Percent of total label value pairs",
    numeric: false,
  },
  {
    disablePadding: false,
    id: "action",
    label: "Action",
    numeric: false,
  }
]as HeadCell[];

export const LABEL_WITH_UNIQUE_VALUES_TABLE_HEADERS = [
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

export const spinnerContainerStyles = (height: string) =>  {
  return {
    width: "100%",
    maxWidth: "100%",
    position: "absolute",
    height: height ?? "50%",
    background: "rgba(255, 255, 255, 0.7)",
    pointerEvents: "none",
    zIndex: 1000,
  };
};

export const SPINNER_TITLE = "Please wait while cardinality stats is calculated. " +
  "This may take some time if the db contains big number of time series";
export const SERIES_CONTENT_TITLE = "Metric names with the highest number of series";
export const LABEL_VALUE_PAIR_CONTENT_TITLE = "Label=value pairs with the highest number of series";
export const LABELS_CONTENT_TITLE = "Labels with the highest number of unique values";
