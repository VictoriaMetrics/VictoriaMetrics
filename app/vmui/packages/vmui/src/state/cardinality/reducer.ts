import dayjs from "dayjs";
import {getQueryStringValue} from "../../utils/query-string";

export interface CardinalityState {
  runQuery: number,
  topN: number
  date: string | null
  match: string | null
  extraLabel: string | null
  focusLabel: string | null
}

export type Action =
  | { type: "SET_TOP_N", payload: number }
  | { type: "SET_DATE", payload: string | null }
  | { type: "SET_MATCH", payload: string | null }
  | { type: "SET_EXTRA_LABEL", payload: string | null }
  | { type: "SET_FOCUS_LABEL", payload: string | null }
  | { type: "RUN_QUERY" }


export const initialState: CardinalityState = {
  runQuery: 0,
  topN: getQueryStringValue("topN", 10) as number,
  date: getQueryStringValue("date", dayjs(new Date()).format("YYYY-MM-DD")) as string,
  focusLabel: getQueryStringValue("focusLabel", "") as string,
  match: (getQueryStringValue("match", []) as string[]).join("&"),
  extraLabel: getQueryStringValue("extra_label", "") as string,
};

export function reducer(state: CardinalityState, action: Action): CardinalityState {
  switch (action.type) {
    case "SET_TOP_N":
      return {
        ...state,
        topN: action.payload
      };
    case "SET_DATE":
      return {
        ...state,
        date: action.payload
      };
    case "SET_MATCH":
      return {
        ...state,
        match: action.payload
      };
    case "SET_EXTRA_LABEL":
      return {
        ...state,
        extraLabel: action.payload
      };
    case "SET_FOCUS_LABEL":
      return {
        ...state,
        focusLabel: action.payload,
      };
    case "RUN_QUERY":
      return {
        ...state,
        runQuery: state.runQuery + 1
      };
    default:
      throw new Error();
  }
}
