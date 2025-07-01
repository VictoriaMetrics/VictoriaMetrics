import { ContextData, ContextType, LogicalPart, LogicalPartPosition, LogicalPartType } from "./types";
import { pipeList } from "./pipes";

const BUILDER_OPERATORS = ["AND", "OR", "NOT"];
const PIPE_NAMES = pipeList.map(p => p.value);

export const splitLogicalParts = (expr: string) => {
  // Replace spaces around the colon (:) with just the colon, removing the spaces
  const input = expr; //.replace(/\s*:\s*/g, ":");
  const parts: LogicalPart[] = [];
  let currentPart = "";
  let separator: undefined | " " | "|" = undefined;
  let isPipePart = false;

  const quotes = ["'", "\"", "`"];
  let insideQuotes = false;
  let expectedQuote = "";

  const openBrackets = ["(", "[", "{"];
  const closeBrackets = [")", "]", "}"];
  const brackets = [...openBrackets, ...closeBrackets];
  let insideBrackets = 0;

  let startIndex = 0;

  for (let i = 0; i < input.length; i++) {
    const char = input[i];

    // Check if the current character is a quote
    if (quotes.includes(char)) {
      const isClosedQuote: boolean = insideQuotes && (char === expectedQuote);
      insideQuotes = !isClosedQuote;
      expectedQuote = isClosedQuote ? "" : char;
    }

    // Check if the current character is a bracket
    if (!insideQuotes && brackets.includes(char)) {
      const dir = openBrackets.includes(char) ? 1 : -1;
      insideBrackets += dir;
    }

    // Check if the current character is a pipe
    if ((!insideQuotes && !insideBrackets && char === "|")) {
      isPipePart = true;
      const countStartSpaces = currentPart.match(/^ */)?.[0].length || 0;
      const countEndSpaces = currentPart.match(/ *$/)?.[0].length || 0;
      pushPart(currentPart, true, [startIndex + countStartSpaces, i - countEndSpaces - 1], parts, separator);
      currentPart = "";
      separator = "|";
      startIndex = i + 1;
      continue;
    }

    // Check if the current character is a space
    if (!isPipePart && !insideQuotes && !insideBrackets && char === " ") {
      const nextStr = input.slice(i).replace(/^\s*/, "");
      const prevStr = input.slice(0, i).replace(/\s*$/, "");
      if (!nextStr.startsWith(":") && !prevStr.endsWith(":")) {
        pushPart(currentPart, false, [startIndex, i - 1], parts, separator);
        separator = " ";
        currentPart = "";
        startIndex = i + 1;
        continue;
      }
    }

    currentPart += char;
  }

  // push the last part
  pushPart(currentPart, isPipePart, [startIndex, input.length], parts, separator);

  return parts;
};

const pushPart = (currentPart: string, isPipePart: boolean, position: LogicalPartPosition, parts: LogicalPart[], separator: LogicalPart["separator"]) => {
  const trimmedPart = currentPart.trim();
  if (!trimmedPart) return;
  const isOperator = BUILDER_OPERATORS.includes(trimmedPart.toUpperCase());
  const pipesTypes = [LogicalPartType.Pipe, LogicalPartType.FilterOrPipe];
  const isPreviousPartPipe = parts.length > 0 && pipesTypes.includes(parts[parts.length - 1].type);

  const getType = () => {
    if (isPreviousPartPipe) return LogicalPartType.FilterOrPipe;
    if (isPipePart) return LogicalPartType.Pipe;
    if (isOperator) return LogicalPartType.Operator;
    return LogicalPartType.Filter;
  };

  parts.push({
    id: parts.length,
    value: trimmedPart,
    position,
    type: getType(),
    separator,
  });
};

export const getContextData = (part: LogicalPart, cursorPos: number): ContextData => {
  const valueBeforeCursor = part.value.substring(0, cursorPos);
  const valueAfterCursor = part.value.substring(cursorPos);

  const metaData: ContextData = {
    valueBeforeCursor,
    valueAfterCursor,
    valueContext: part.value,
    contextType: ContextType.Unknown,
  };

  // Determine a context type based on a logical part type
  determineContextType(part, valueBeforeCursor, valueAfterCursor, metaData);

  // Clean up quotes in valueContext
  metaData.valueContext = metaData.valueContext.replace(/^["']|["']$/g, "");

  return metaData;
};

/** Helper function to determine if a string starts with any of the pipe names */
const startsWithPipe = (value: string): boolean => {
  return PIPE_NAMES.some(p => value.startsWith(p));
};

/** Helper function to check for colon presence */
const hasNoColon = (before: string, after: string): boolean => {
  return !before.includes(":") && !after.includes(":");
};

/** Helper function to extract filter name and update metadata for filter values */
const handleFilterValue = (valueBeforeCursor: string, metaData: ContextData): void => {
  const [filterName, ...filterValue] = valueBeforeCursor.split(":");
  metaData.contextType = ContextType.FilterValue;
  metaData.filterName = filterName;
  const enhanceOperators = ["=", "-", "!"] as const;
  const enhanceOperator = enhanceOperators.find(op => op === filterValue[0]);
  if (enhanceOperator) {
    metaData.valueContext = filterValue.slice(1).join(":");
    metaData.operator = `:${enhanceOperator}`;
  } else {
    metaData.valueContext = filterValue.join(":");
    metaData.operator = ":";
  }
};

/** Function to determine a context type based on part type and value */
const determineContextType = (
  part: LogicalPart,
  valueBeforeCursor: string,
  valueAfterCursor: string,
  metaData: ContextData
): void => {
  switch (part.type) {
    case LogicalPartType.Filter:
      handleFilterType(valueBeforeCursor, valueAfterCursor, metaData);
      break;

    case LogicalPartType.Pipe:
      metaData.contextType = startsWithPipe(part.value)
        ? ContextType.PipeValue
        : ContextType.PipeName;
      break;

    case LogicalPartType.FilterOrPipe:
      handleFilterOrPipeType(part.value, valueBeforeCursor, metaData);
      break;
  }
};

/** Handle filter type context determination */
const handleFilterType = (
  valueBeforeCursor: string,
  valueAfterCursor: string,
  metaData: ContextData
): void => {
  if (hasNoColon(valueBeforeCursor, valueAfterCursor)) {
    metaData.contextType = ContextType.FilterUnknown;
  } else if (valueBeforeCursor.includes(":")) {
    handleFilterValue(valueBeforeCursor, metaData);
  } else {
    metaData.contextType = ContextType.FilterName;
  }
};

/** Handle FilterOrPipeType context determination */
const handleFilterOrPipeType = (
  value: string,
  valueBeforeCursor: string,
  metaData: ContextData
): void => {
  if (startsWithPipe(value)) {
    metaData.contextType = ContextType.PipeValue;
  } else if (valueBeforeCursor.includes(":")) {
    handleFilterValue(valueBeforeCursor, metaData);
  } else {
    metaData.contextType = ContextType.FilterOrPipeName;
  }
};
