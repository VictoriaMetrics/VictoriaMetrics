import { getFromStorage, saveToStorage } from "../../utils/storage";
import { getQueryArray } from "../../utils/query-string";
import { setQueriesToStorage } from "../../pages/CustomPanel/QueryHistory/utils";

export interface QueryHistoryType {
  index: number;
  values: string[];
}

export interface QueryState {
  query: string[];
  queryHistory: QueryHistoryType[];
  autocomplete: boolean;
  quickAutocomplete: boolean;
}

export type QueryAction =
  | { type: "SET_QUERY", payload: string[] }
  | { type: "SET_QUERY_HISTORY_BY_INDEX", payload: {value: QueryHistoryType, queryNumber: number} }
  | { type: "SET_QUERY_HISTORY", payload: QueryHistoryType[] }
  | { type: "TOGGLE_AUTOCOMPLETE"}
  | { type: "SET_QUICK_AUTOCOMPLETE", payload: boolean}

const query = getQueryArray();
export const initialQueryState: QueryState = {
  query,
  queryHistory: query.map(q => ({ index: 0, values: [q] })),
  autocomplete: getFromStorage("AUTOCOMPLETE") as boolean || false,
  quickAutocomplete: false,
};

export function reducer(state: QueryState, action: QueryAction): QueryState {
  switch (action.type) {
    case "SET_QUERY":
      return {
        ...state,
        query: action.payload.map(q => q)
      };
    case "SET_QUERY_HISTORY":
      setQueriesToStorage(action.payload);
      return {
        ...state,
        queryHistory: action.payload
      };
    case "SET_QUERY_HISTORY_BY_INDEX":
      state.queryHistory.splice(action.payload.queryNumber, 1, action.payload.value);
      return {
        ...state,
        queryHistory: state.queryHistory
      };
    case "TOGGLE_AUTOCOMPLETE":
      saveToStorage("AUTOCOMPLETE", !state.autocomplete);
      return {
        ...state,
        autocomplete: !state.autocomplete
      };
    case "SET_QUICK_AUTOCOMPLETE":
      return {
        ...state,
        quickAutocomplete: action.payload
      };
    default:
      throw new Error();
  }
}
