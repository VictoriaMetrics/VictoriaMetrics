import {DisplayType} from "../../components/Home/Configurator/DisplayTypeSwitch";
import {TimeParams, TimePeriod} from "../../types";
import {dateFromSeconds, getDurationFromPeriod, getTimeperiodForDuration} from "../../utils/time";
import {getFromStorage} from "../../utils/storage";
import {getDefaultServer} from "../../utils/default-server-url";
import {getQueryStringValue} from "../../utils/query-string";

export interface TimeState {
  duration: string;
  period: TimeParams;
}

export interface AppState {
  serverUrl: string;
  displayType: DisplayType;
  query: string;
  time: TimeState;
  queryControls: {
    autoRefresh: boolean;
    autocomplete: boolean
  }
}

export type Action =
    | { type: "SET_DISPLAY_TYPE", payload: DisplayType }
    | { type: "SET_SERVER", payload: string }
    | { type: "SET_QUERY", payload: string }
    | { type: "SET_DURATION", payload: string }
    | { type: "SET_UNTIL", payload: Date }
    | { type: "SET_PERIOD", payload: TimePeriod }
    | { type: "RUN_QUERY"}
    | { type: "RUN_QUERY_TO_NOW"}
    | { type: "TOGGLE_AUTOREFRESH"}
    | { type: "TOGGLE_AUTOCOMPLETE"}

const duration = getQueryStringValue("g0.range_input", "1h") as string;
const endInput = getQueryStringValue("g0.end_input", undefined) as Date | undefined;

export const initialState: AppState = {
  serverUrl: getDefaultServer(),
  displayType: "chart",
  query: getQueryStringValue("g0.expr", getFromStorage("LAST_QUERY") as string || "\n") as string, // demo_memory_usage_bytes
  time: {
    duration,
    period: getTimeperiodForDuration(duration, endInput && new Date(endInput))
  },
  queryControls: {
    autoRefresh: false,
    autocomplete: getFromStorage("AUTOCOMPLETE") as boolean || false
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
        query: action.payload
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
