/* eslint max-lines: 0 */
import {DisplayType} from "../../components/Home/Configurator/DisplayTypeSwitch";
import {TimeParams, TimePeriod} from "../../types";
import {
  dateFromSeconds,
  formatDateToLocal,
  getDateNowUTC,
  getDurationFromPeriod,
  getTimeperiodForDuration,
  getDurationFromMilliseconds
} from "../../utils/time";
import {getFromStorage} from "../../utils/storage";
import {getDefaultServer} from "../../utils/default-server-url";
import {getQueryArray, getQueryStringValue} from "../../utils/query-string";
import dayjs from "dayjs";

export interface TimeState {
  duration: string;
  period: TimeParams;
}

export interface QueryHistory {
  index: number,
  values: string[]
}

export interface AppState {
  serverUrl: string;
  displayType: DisplayType;
  query: string[];
  time: TimeState;
  queryHistory: QueryHistory[],
  queryControls: {
    autoRefresh: boolean;
    autocomplete: boolean,
    nocache: boolean
  }
}

export type Action =
    | { type: "SET_DISPLAY_TYPE", payload: DisplayType }
    | { type: "SET_SERVER", payload: string }
    | { type: "SET_QUERY", payload: string[] }
    | { type: "SET_QUERY_HISTORY_BY_INDEX", payload: {value: QueryHistory, queryNumber: number} }
    | { type: "SET_QUERY_HISTORY", payload: QueryHistory[] }
    | { type: "SET_DURATION", payload: string }
    | { type: "SET_UNTIL", payload: Date }
    | { type: "SET_FROM", payload: Date }
    | { type: "SET_PERIOD", payload: TimePeriod }
    | { type: "RUN_QUERY"}
    | { type: "RUN_QUERY_TO_NOW"}
    | { type: "TOGGLE_AUTOREFRESH"}
    | { type: "TOGGLE_AUTOCOMPLETE"}
    | { type: "NO_CACHE"}

const duration = getQueryStringValue("g0.range_input", "1h") as string;
const endInput = formatDateToLocal(getQueryStringValue("g0.end_input", getDateNowUTC()) as Date);
const query = getQueryArray();

export const initialState: AppState = {
  serverUrl: getDefaultServer(),
  displayType: getQueryStringValue("g0.tab", "chart") as DisplayType,
  query: query, // demo_memory_usage_bytes
  queryHistory: query.map(q => ({index: 0, values: [q]})),
  time: {
    duration,
    period: getTimeperiodForDuration(duration, new Date(endInput))
  },
  queryControls: {
    autoRefresh: false,
    autocomplete: getFromStorage("AUTOCOMPLETE") as boolean || false,
    nocache: getFromStorage("NO_CACHE") as boolean || false,
  }
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case "SET_DISPLAY_TYPE":
      return {
        ...state,
        displayType: action.payload
      };
    case "SET_SERVER":
      return {
        ...state,
        serverUrl: action.payload
      };
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
    case "SET_DURATION":
      return {
        ...state,
        time: {
          ...state.time,
          duration: action.payload,
          period: getTimeperiodForDuration(action.payload, dateFromSeconds(state.time.period.end))
        }
      };
    case "SET_UNTIL":
      return {
        ...state,
        time: {
          ...state.time,
          period: getTimeperiodForDuration(state.time.duration, action.payload)
        }
      };
    case "SET_FROM":
      // eslint-disable-next-line no-case-declarations
      const durationFrom = getDurationFromMilliseconds(state.time.period.end*1000 - action.payload.valueOf());
      return {
        ...state,
        queryControls: {
          ...state.queryControls,
          autoRefresh: false // since we're considering this to action to be fired from period selection on chart
        },
        time: {
          ...state.time,
          duration: durationFrom,
          period: getTimeperiodForDuration(durationFrom, dayjs(state.time.period.end*1000).toDate())
        }
      };
    case "SET_PERIOD":
      // eslint-disable-next-line no-case-declarations
      const duration = getDurationFromPeriod(action.payload);
      return {
        ...state,
        queryControls: {
          ...state.queryControls,
          autoRefresh: false // since we're considering this to action to be fired from period selection on chart
        },
        time: {
          ...state.time,
          duration,
          period: getTimeperiodForDuration(duration, action.payload.to)
        }
      };
    case "TOGGLE_AUTOREFRESH":
      return {
        ...state,
        queryControls: {
          ...state.queryControls,
          autoRefresh: !state.queryControls.autoRefresh
        }
      };
    case "TOGGLE_AUTOCOMPLETE":
      return {
        ...state,
        queryControls: {
          ...state.queryControls,
          autocomplete: !state.queryControls.autocomplete
        }
      };
    case "NO_CACHE":
      return {
        ...state,
        queryControls: {
          ...state.queryControls,
          nocache: !state.queryControls.nocache
        }
      };
    case "RUN_QUERY":
      return {
        ...state,
        time: {
          ...state.time,
          period: getTimeperiodForDuration(state.time.duration, dateFromSeconds(state.time.period.end))
        }
      };
    case "RUN_QUERY_TO_NOW":
      return {
        ...state,
        time: {
          ...state.time,
          period: getTimeperiodForDuration(state.time.duration)
        }
      };
    default:
      throw new Error();
  }
}
