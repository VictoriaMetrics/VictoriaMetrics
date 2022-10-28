import { getFromStorage } from "../../utils/storage";
import { getQueryArray } from "../../utils/query-string";

export interface QueryHistory {
  index: number;
  values: string[];
}

export interface QueryState {
  query: string[];
  queryHistory: QueryHistory[];
  autocomplete: boolean;

}

export type QueryAction =
  | { type: "SET_QUERY", payload: string[] }
  | { type: "SET_QUERY_HISTORY_BY_INDEX", payload: {value: QueryHistory, queryNumber: number} }
  | { type: "SET_QUERY_HISTORY", payload: QueryHistory[] }
  | { type: "TOGGLE_AUTOCOMPLETE"}

const query = getQueryArray();
export const initialQueryState: QueryState = {
  query,
  queryHistory: query.map(q => ({ index: 0, values: [q] })),
  autocomplete: getFromStorage("AUTOCOMPLETE") as boolean || false,
};

export function reducer(state: QueryState, action: QueryAction): QueryState {
  switch (action.type) {
    case "SET_QUERY":
      return {
        ...state,
        query: action.payload.map(q => q)
      };
    case "SET_QUERY_HISTORY":
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
      return {
        ...state,
        autocomplete: !state.autocomplete
      };
    default:
      throw new Error();
  }
}
