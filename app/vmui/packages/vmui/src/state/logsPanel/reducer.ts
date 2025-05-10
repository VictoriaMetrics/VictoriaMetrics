import { getFromStorage, saveToStorage } from "../../utils/storage";
import { LogsFiledValues } from "../../api/types";
import { AUTOCOMPLETE_LIMITS } from "../../constants/queryAutocomplete";

export interface LogsState {
  markdownParsing: boolean;
  ansiParsing: boolean;
  autocompleteCache: Map<string, LogsFiledValues[]>;
}

export type LogsAction =
  | { type: "SET_MARKDOWN_PARSING", payload: boolean }
  | { type: "SET_ANSI_PARSING", payload: boolean }
  | { type: "SET_AUTOCOMPLETE_CACHE", payload: { key: string, value: LogsFiledValues[] } }


export const initialLogsState: LogsState = {
  markdownParsing: getFromStorage("LOGS_MARKDOWN") === "true",
  ansiParsing: getFromStorage("LOGS_ANSI") === "true",
  autocompleteCache: new Map<string, LogsFiledValues[]>(),
};

export function reducer(state: LogsState, action: LogsAction): LogsState {
  switch (action.type) {
    case "SET_MARKDOWN_PARSING":
      saveToStorage("LOGS_MARKDOWN", `${ action.payload}`);
      return {
        ...state,
        markdownParsing: action.payload
      };
    case "SET_ANSI_PARSING":
      saveToStorage("LOGS_ANSI", `${ action.payload}`);
      return {
        ...state,
        ansiParsing: action.payload
      };
    case "SET_AUTOCOMPLETE_CACHE": {
      if (state.autocompleteCache.size >= AUTOCOMPLETE_LIMITS.cacheLimit) {
        const firstKey = state.autocompleteCache.keys().next().value;
        firstKey && state.autocompleteCache.delete(firstKey);
      }
      state.autocompleteCache.set(action.payload.key, action.payload.value);

      return {
        ...state,
        autocompleteCache: state.autocompleteCache,
      };
    }
    default:
      throw new Error();
  }
}
