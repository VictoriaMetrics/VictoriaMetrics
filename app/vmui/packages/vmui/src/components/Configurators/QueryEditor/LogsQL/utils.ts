import { ContextData } from "./types";

const getStreamFieldQuery = (valueContext: string) => {
  if (valueContext.includes("=")) {
    const [fieldName, fieldValue] = valueContext.split("=");
    if (fieldValue) {
      return `_stream:${fieldName}=~${fieldValue}.*"}`;
    }
  }

  return "*";
};

const getLastPartUntilDelimiter = (value: string, delimiter: string) => {
  const lastIndexOfDelimiter = value.lastIndexOf(delimiter);
  return lastIndexOfDelimiter !== -1 ? value.slice(0, lastIndexOfDelimiter) : "";
};

const getDateQuery = (contextData: ContextData) => {
  let fieldValue = "";
  if (contextData.valueContext.includes(":")) {
    fieldValue = getLastPartUntilDelimiter(contextData.valueContext, ":");
  } else if (contextData.valueContext.includes("-")) {
    fieldValue = getLastPartUntilDelimiter(contextData.valueContext, "-");
  }
  return fieldValue ? `${contextData.filterName}:${fieldValue}` : "*";
};

/**
 * Generates a query string based on the provided context data.
 *
 * The function processes the input based on the `filterName` property:
 *
 * - If `filterName` is `_msg` or `_stream_id`, the query cannot be generated specifically,
 *   so a wildcard query (`"*"`) is returned.
 *
 * - If `filterName` is `_stream`, the query is generated using regexp (`{type=~"value.*"}`).
 *
 * - If `filterName` is `_time`, a simplified query is created by trimming the value up
 *   to the first occurrence of a delimiter such as `-` or `:`.
 *
 * - For all other values of `filterName`, a prefix query is returned using
 *   the `query` value with a `*` appended (e.g., `"value*"`).
 *
 * @param {ContextData} contextData - The context object containing query parameters and metadata.
 * @returns {string} The generated query string.
 */
export const generateQuery = (contextData: ContextData): string => {
  let fieldQuery = "";
  if (!contextData.filterName || !contextData.query || ["_msg", "_stream_id"].includes(contextData.filterName)) {
    fieldQuery = "*";
  } else if ("_stream" === contextData.filterName) {
    fieldQuery = getStreamFieldQuery(contextData.valueContext);
  } else if ("_time" === contextData.filterName) {
    fieldQuery = getDateQuery(contextData);
  } else {
    fieldQuery = `${contextData.filterName}:${contextData.valueContext}*`;
  }

  return contextData.queryBeforeIncompleteFilter ? `${contextData.queryBeforeIncompleteFilter} ${fieldQuery}` : fieldQuery;
};