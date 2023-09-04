import { TimeParams, TimePeriod } from "../../types";
import {
  dateFromSeconds,
  formatDateToLocal,
  getDateNowUTC,
  getDurationFromPeriod,
  getTimeperiodForDuration,
  getRelativeTime,
  setTimezone
} from "../../utils/time";
import { getQueryStringValue } from "../../utils/query-string";
import dayjs from "dayjs";
import { getFromStorage, saveToStorage } from "../../utils/storage";

export interface TimeState {
  duration: string;
  period: TimeParams;
  relativeTime?: string;
  timezone: string;
}

export type TimeAction =
  | { type: "SET_DURATION", payload: string }
  | { type: "SET_RELATIVE_TIME", payload: {id: string, duration: string, until: Date} }
  | { type: "SET_PERIOD", payload: TimePeriod }
  | { type: "RUN_QUERY"}
  | { type: "RUN_QUERY_TO_NOW"}
  | { type: "SET_TIMEZONE", payload: string }

const timezone = getFromStorage("TIMEZONE") as string || dayjs.tz.guess();
setTimezone(timezone);

const defaultDuration = getQueryStringValue("g0.range_input") as string;

const { duration, endInput, relativeTimeId } = getRelativeTime({
  defaultDuration: defaultDuration || "1h",
  defaultEndInput: formatDateToLocal(getQueryStringValue("g0.end_input", getDateNowUTC()) as string),
  relativeTimeId: defaultDuration ? getQueryStringValue("g0.relative_time", "none") as string : undefined
});

export const initialTimeState: TimeState = {
  duration,
  period: getTimeperiodForDuration(duration, endInput),
  relativeTime: relativeTimeId,
  timezone,
};


export function reducer(state: TimeState, action: TimeAction): TimeState {
  switch (action.type) {
    case "SET_DURATION":
      return {
        ...state,
        duration: action.payload,
        period: getTimeperiodForDuration(action.payload, dateFromSeconds(state.period.end)),
        relativeTime: "none"
      };
    case "SET_RELATIVE_TIME":
      return {
        ...state,
        duration: action.payload.duration,
        period: getTimeperiodForDuration(action.payload.duration, action.payload.until),
        relativeTime: action.payload.id,
      };
    case "SET_PERIOD":
      // eslint-disable-next-line no-case-declarations
      const durationPeriod = getDurationFromPeriod(action.payload);
      return {
        ...state,
        duration: durationPeriod,
        period: getTimeperiodForDuration(durationPeriod, action.payload.to),
        relativeTime: "none"
      };
    case "RUN_QUERY":
      // eslint-disable-next-line no-case-declarations
      const { duration: durationRunQuery, endInput } = getRelativeTime({
        relativeTimeId: state.relativeTime,
        defaultDuration: state.duration,
        defaultEndInput: dateFromSeconds(state.period.end),
      });
      return {
        ...state,
        period: getTimeperiodForDuration(durationRunQuery, endInput)
      };
    case "RUN_QUERY_TO_NOW":
      return {
        ...state,
        period: getTimeperiodForDuration(state.duration)
      };
    case "SET_TIMEZONE":
      setTimezone(action.payload);
      saveToStorage("TIMEZONE", action.payload);
      return {
        ...state,
        timezone: action.payload
      };
    default:
      throw new Error();
  }
}
