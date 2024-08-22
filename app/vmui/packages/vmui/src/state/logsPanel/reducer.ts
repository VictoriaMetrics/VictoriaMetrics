import { getFromStorage, saveToStorage } from "../../utils/storage";

export interface LogsState {
  markdownParsing: boolean;
}

export type LogsAction =
  | { type: "SET_MARKDOWN_PARSING", payload: boolean }


export const initialLogsState: LogsState = {
  markdownParsing: getFromStorage("LOGS_MARKDOWN") === "true",
};

export function reducer(state: LogsState, action: LogsAction): LogsState {
  switch (action.type) {
    case "SET_MARKDOWN_PARSING":
      saveToStorage("LOGS_MARKDOWN", `${ action.payload}`);
      return {
        ...state,
        markdownParsing: action.payload
      };
    default:
      throw new Error();
  }
}
