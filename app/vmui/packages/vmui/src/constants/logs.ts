import { DATE_TIME_FORMAT } from "./date";

export const LOGS_ENTRIES_LIMIT = 50;
export const LOGS_BARS_VIEW = 100;
export const LOGS_LIMIT_HITS = 5;

// "Ungrouped" is a string that is used as a value for the "groupBy" parameter.
export const WITHOUT_GROUPING = "Ungrouped";

// Default values for the logs configurators.
export const LOGS_GROUP_BY = "_stream";
export const LOGS_DISPLAY_FIELDS = "_msg";
export const LOGS_DATE_FORMAT = `${DATE_TIME_FORMAT}.SSS`;

// URL parameters for the logs page.
export const LOGS_URL_PARAMS = {
  GROUP_BY: "groupBy",
  DISPLAY_FIELDS: "displayFields",
  NO_WRAP_LINES: "noWrapLines",
  COMPACT_GROUP_HEADER: "compactGroupHeader",
  DATE_FORMAT: "dateFormat",
};
