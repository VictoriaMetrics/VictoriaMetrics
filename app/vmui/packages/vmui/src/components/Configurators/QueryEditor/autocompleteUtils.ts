/**
 * Utility functions for query editor autocomplete functionality
 */
import { QueryContextType } from "../../../types";
import { escapeRegexp, hasUnclosedQuotes } from "../../../utils/regexp";

/**
 * Extracts the last part of an expression that's relevant for auto-suggestion
 * @param beforeCursor The text before the cursor position
 * @returns The relevant part of the expression for auto-suggestion
 */
export function getExprLastPart(beforeCursor: string): string {
  const regexpSplit =
    /\s(or|and|unless|default|ifnot|if|group_left|group_right)\s|}|\+|\|-|\*|\/|\^/i;
  const parts = beforeCursor.split(regexpSplit);
  const lastPart = parts[parts.length - 1].trim();

  // Check if we're inside a function's parameters
  const functionRegex = /.*\(([^)]*)$/;
  const functionMatch = lastPart.match(functionRegex);

  if (functionMatch && functionMatch[1]) {
    const params = functionMatch[1];

    if (params.lastIndexOf("{") > params.lastIndexOf("}")) {
      const wordMatch = params.match(/([\w_.:]+)\{[^}]*$/);
      return wordMatch ? wordMatch[0] : lastPart;
    }

    const lastCommaPos = params.lastIndexOf(",");
    if (lastCommaPos !== -1) {
      return params.substring(lastCommaPos + 1).trim();
    }

    return params;
  }

  return lastPart;
}

/**
 * Extracts the current word or value at the cursor position for auto-suggestion matching
 * @param beforeCursor The text before the cursor position
 * @returns The current word or value at the cursor position
 */
export function getValueByContext(beforeCursor: string): string {
  const wordMatch = beforeCursor.match(/([\w_.:]+(?![},]))$/);
  return wordMatch ? wordMatch[0] : "";
}

/**
 * Determines if auto-suggestion should be suppressed based on the query
 * @param value The query value
 * @returns Whether auto-suggestion should be suppressed
 */
export function shouldSuppressAutoSuggestion(value: string): boolean {
  const pattern =
    /([{(),+\-*/^]|\b(?:or|and|unless|default|ifnot|if|group_left|group_right|by|without|on|ignoring)\b)/i;
  const parts = value.split(/\s+/);
  const partsCount = parts.length;
  const lastPart = parts[partsCount - 1];
  const preLastPart = parts[partsCount - 2];

  const hasEmptyPartAndQuotes = !lastPart && hasUnclosedQuotes(value);
  const suppressPreLast =
    (!lastPart || parts.length > 1) && !pattern.test(preLastPart);
  return hasEmptyPartAndQuotes || suppressPreLast;
}

/**
 * Determines the context type for auto-suggestion based on the query
 * @param beforeCursor The text before the cursor position
 * @param metric The current metric name
 * @param label The current label name
 * @returns The context type for auto-suggestion
 */
export function getContext(
  beforeCursor: string,
  metric: string = "",
  label: string = ""
): QueryContextType {
  const valueBeforeCursor = beforeCursor.trim();
  const endOfClosedBrackets = ["}", ")"].some((char) =>
    valueBeforeCursor.endsWith(char)
  );
  const endOfClosedQuotes =
    !hasUnclosedQuotes(valueBeforeCursor) &&
    ["`", "'", '"'].some((char) => valueBeforeCursor.endsWith(char));
  if (
    !valueBeforeCursor ||
    endOfClosedBrackets ||
    endOfClosedQuotes ||
    shouldSuppressAutoSuggestion(valueBeforeCursor)
  ) {
    return QueryContextType.empty;
  }

  const labelRegexp = /(?:by|without|on|ignoring)\s*\(\s*[^)]*$|\{[^}]*$/i;
  const patternLabelValue = `(${escapeRegexp(metric)})?{?.+${escapeRegexp(
    label
  )}(=|!=|=~|!~)"?([^"]*)$`;
  const labelValueRegexp = new RegExp(patternLabelValue, "g");

  switch (true) {
    case labelValueRegexp.test(valueBeforeCursor):
      return QueryContextType.labelValue;
    case labelRegexp.test(valueBeforeCursor):
      return QueryContextType.label;
    default:
      return QueryContextType.metricsql;
  }
}
