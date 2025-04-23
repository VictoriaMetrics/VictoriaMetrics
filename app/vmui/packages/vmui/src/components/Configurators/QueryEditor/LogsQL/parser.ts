import { ContextData, ContextType, LogicalPart, LogicalPartPosition, LogicalPartType } from "./types";
import { pipeList } from "./pipes";

const BUILDER_OPERATORS = ["AND", "OR", "NOT"];
const PIPE_NAMES = pipeList.map(p => p.value);

export const splitLogicalParts = (expr: string) => {
  // Replace spaces around the colon (:) with just the colon, removing the spaces
  const input = expr; //.replace(/\s*:\s*/g, ":");
  const parts: LogicalPart[] = [];
  let currentPart = "";
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
      pushPart(currentPart, true, [startIndex + countStartSpaces, i - countEndSpaces - 1], parts);
      currentPart = "";
      startIndex = i + 1;
      continue;
    }

    // Check if the current character is a space
    if (!isPipePart && !insideQuotes && !insideBrackets && char === " ") {
      const nextStr = input.slice(i).replace(/^\s*/, "");
      const prevStr = input.slice(0, i).replace(/\s*$/, "");
      if (!nextStr.startsWith(":") && !prevStr.endsWith(":")) {
        pushPart(currentPart, false, [startIndex, i - 1], parts);
        currentPart = "";
        startIndex = i + 1;
        continue;
      }
    }

    currentPart += char;
  }

  // push the last part
  pushPart(currentPart, isPipePart, [startIndex, input.length], parts);

  return parts;
};

const pushPart = (currentPart: string, isPipePart: boolean, position: LogicalPartPosition, parts: LogicalPart[]) => {
  const trimmedPart = currentPart.trim();
  if (!trimmedPart) return;
  const isOperator = BUILDER_OPERATORS.includes(trimmedPart.toUpperCase());
  parts.push({
    id: parts.length,
    value: trimmedPart,
    position,
    type: isPipePart
      ? LogicalPartType.Pipe
      : isOperator ? LogicalPartType.Operator : LogicalPartType.Filter,
  });
};

export const getContextData = (part: LogicalPart, cursorPos: number) => {
  const valueBeforeCursor = part.value.substring(0, cursorPos);
  const valueAfterCursor = part.value.substring(cursorPos);
  const enhanceOperators = ["=", "-", "!", "~", "<", ">", "<=", ">="] as const;

  const metaData: ContextData = {
    valueBeforeCursor,
    valueAfterCursor,
    valueContext: part.value,
    contextType: ContextType.Unknown,
  };

  if (part.type === LogicalPartType.Filter) {
    const noColon = !valueBeforeCursor.includes(":") && !valueAfterCursor.includes(":");
    if (noColon) {
      metaData.contextType = ContextType.FilterUnknown;
    } else if (valueBeforeCursor.includes(":")) {
      const [filterName, ...filterValue] = valueBeforeCursor.split(":");
      metaData.contextType = ContextType.FilterValue;
      metaData.filterName = filterName;
      const enhanceOperator = enhanceOperators.find(op => op === filterValue[0]);
      if(enhanceOperator){
        metaData.valueContext = filterValue.slice(1).join(":");
        metaData.operator = `:${enhanceOperator}`;
      } else {
        metaData.valueContext = filterValue.join(":");
        metaData.operator = ":";
      }
    } else {
      metaData.contextType = ContextType.FilterName;
    }
  } else if (part.type === LogicalPartType.Pipe) {
    const valueStartWithPipe = PIPE_NAMES.some(p => part.value.startsWith(p));
    metaData.contextType = valueStartWithPipe ? ContextType.PipeValue : ContextType.PipeName;
  }

  metaData.valueContext = metaData.valueContext.replace(/^["']|["']$/g, "");
  return metaData;
};
