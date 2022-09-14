import {getQueryStringValue} from "../../utils/query-string";

export interface TopQueriesState {
  maxLifetime: string,
  topN: number | null,
  runQuery: number
}

export type Action =
  | { type: "SET_TOP_N", payload: number | null }
  | { type: "SET_MAX_LIFE_TIME", payload: string }
  | { type: "SET_RUN_QUERY" }


export const initialState: TopQueriesState = {
  topN: getQueryStringValue("topN", null) as number,
  maxLifetime: getQueryStringValue("maxLifetime", "") as string,
  runQuery: 0
};

export function reducer(state: TopQueriesState, action: Action): TopQueriesState {
  switch (action.type) {
    case "SET_TOP_N":
      return {
        ...state,
        topN: action.payload
      };
    case "SET_MAX_LIFE_TIME":
      return {
        ...state,
        maxLifetime: action.payload
      };
    case "SET_RUN_QUERY":
      return {
        ...state,
        runQuery: state.runQuery + 1
      };
    default:
      throw new Error();
  }
}
