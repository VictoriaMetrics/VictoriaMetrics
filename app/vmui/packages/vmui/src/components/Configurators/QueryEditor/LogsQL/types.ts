export enum LogicalPartType {
  Filter = "Filter",
  Pipe = "Pipe",
  Operator = "Operator",
  FilterOrPipe = "FilterOrPipe",
}

export type LogicalPartPosition = [start: number, end: number];

export type LogicalPartSeparator = " " | "|";

export interface LogicalPart {
  id: number;
  value: string;
  type: LogicalPartType;
  position: LogicalPartPosition;
  separator?: LogicalPartSeparator;
}

export interface ContextData {
  valueBeforeCursor: string;
  valueAfterCursor: string;
  contextType: ContextType;
  valueContext: string;
  filterName?: string;
  query?: string;
  queryBeforeIncompleteFilter?: string;
  separator?: LogicalPartSeparator;
  operator?: ":" | ":!" | ":-" | ":=" | ":~" | ":<" | ":>" | ":<=" | ":>=";
}

export enum ContextType {
  FilterName = "FilterName",
  FilterUnknown = "FilterUnknown",
  FilterValue = "FilterValue",
  PipeName = "Pipes",
  PipeValue = "PipeValue",
  Unknown = "Unknown",
  FilterOrPipeName = "FilterOrPipeName",
}
