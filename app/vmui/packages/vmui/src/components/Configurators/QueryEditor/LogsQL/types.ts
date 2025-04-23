export enum LogicalPartType {
  Filter = "Filter",
  Pipe = "Pipe",
  Operator = "Operator",
}

export type LogicalPartPosition = [start: number, end: number];

export interface LogicalPart {
  id: number;
  value: string;
  type: LogicalPartType;
  position: LogicalPartPosition;
}

export interface ContextData {
  valueBeforeCursor: string;
  valueAfterCursor: string;
  contextType: ContextType;
  valueContext: string;
  filterName?: string;
  query?: string;
  queryBeforeIncompleteFilter?: string;
  operator?: ":" | ":!" | ":-" | ":=" | ":~" | ":<" | ":>" | ":<=" | ":>=";
}

export enum ContextType {
  FilterName = "FilterName",
  FilterUnknown = "FilterUnknown",
  FilterValue = "FilterValue",
  PipeName = "Pipes",
  PipeValue = "PipeValue",
  Unknown = "Unknown",
}
