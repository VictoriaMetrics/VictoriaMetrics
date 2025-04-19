import { HeadCell } from "./Table/types";

const actionHeader = {
  id: "action",
  label: "",
  modifiers: ["action"],
};

const diffHeader = {
  id: "diff",
  label: "1d",
  info: "Shows the absolute difference compared to the previous day.",
  sortable: true,
  modifiers: ["compact"],
};

const diffPercentHeader = {
  id: "diffPercent",
  label: "1d %",
  info: "Shows the percentage difference compared to the previous day.",
  sortable: true,
  modifiers: ["compact"],
};

const valueHeader = {
  id: "value",
  label: "Number of series",
  sortable: true,
  modifiers: ["compact"],
};

export const METRIC_NAMES_HEADERS: HeadCell[] = [
  {
    id: "name",
    label: "Metric name",
    sortable: true,
  },
  valueHeader,
  {
    id: "requestsCount",
    label: "Requests count",
    sortable: true,
    modifiers: ["compact"],
    info: "The number of times this metric was queried since stats collection began.",
  },
  {
    id: "lastRequestTimestamp",
    label: "Last request",
    sortable: true,
    modifiers: ["compact"],
    info: "The last time this metric was used in a query since stats collection began.",
  },
  diffHeader,
  diffPercentHeader,
  {
    id: "percentage",
    label: "Share in total",
    info: "Shows the share of a metric to the total number of series"
  },
  actionHeader
];

export const LABEL_NAMES_HEADERS: HeadCell[] = [
  {
    id: "name",
    label: "Label name",
    sortable: true,
  },
  valueHeader,
  diffHeader,
  diffPercentHeader,
  {
    id: "percentage",
    label: "Share in total",
    info: "Shows the share of the label to the total number of series"
  },
  actionHeader,
];

export const FOCUS_LABEL_VALUES_HEADERS: HeadCell[] = [
  {
    id: "name",
    label: "Label value",
    sortable: true,
  },
  valueHeader,
  diffHeader,
  diffPercentHeader,
  {
    id: "percentage",
    label: "Share in total",
  },
  actionHeader,
];

export const LABEL_VALUE_PAIRS_HEADERS: HeadCell[] = [
  {
    id: "name",
    label: "Label=value pair",
    sortable: true,
  },
  valueHeader,
  diffHeader,
  diffPercentHeader,
  {
    id: "percentage",
    label: "Share in total",
    info: "Shows the share of the label value pair to the total number of series"
  },
  actionHeader,
];

export const LABEL_NAMES_WITH_UNIQUE_VALUES_HEADERS: HeadCell[] = [
  {
    id: "name",
    label: "Label name",
    sortable: true,
  },
  {
    id: "value",
    label: "Number of unique values",
    sortable: true,
    modifiers: ["compact"],
  },
  diffHeader,
  diffPercentHeader,
  actionHeader
];
