import {HeadCell} from "../Table/types";

export const METRIC_NAMES_HEADERS = [
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

export const LABEL_NAMES_HEADERS = [
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
]as HeadCell[];

export const LABELS_WITH_UNIQUE_VALUES_HEADERS = [
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

export const LABEL_WITH_HIGHEST_SERIES_HEADERS = [
  {
    disablePadding: false,
    id: "name",
    label: "Value name",
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
