import {RelativeTimeOption, TimeParams, TimePeriod} from "../types";
import dayjs, {UnitTypeShort} from "dayjs";
import duration from "dayjs/plugin/duration";
import utc from "dayjs/plugin/utc";
import {getQueryStringValue} from "./query-string";

dayjs.extend(duration);
dayjs.extend(utc);

const MAX_ITEMS_PER_CHART = window.innerWidth / 4;

export const limitsDurations = {min: 1, max: 1.578e+11}; // min: 1 ms, max: 5 years

export const dateIsoFormat = "YYYY-MM-DD[T]HH:mm:ss";

export const supportedDurations = [
  {long: "days", short: "d", possible: "day"},
  {long: "weeks", short: "w", possible: "week"},
  {long: "months", short: "M", possible: "mon"},
  {long: "years", short: "y", possible: "year"},
  {long: "hours", short: "h", possible: "hour"},
  {long: "minutes", short: "m", possible: "min"},
  {long: "seconds", short: "s", possible: "sec"},
  {long: "milliseconds", short: "ms", possible: "millisecond"}
];

const shortDurations = supportedDurations.map(d => d.short);

export const roundToMilliseconds = (num: number): number => Math.round(num*1000)/1000;

export const isSupportedDuration = (str: string): Partial<Record<UnitTypeShort, string>> | undefined => {

  const digits = str.match(/\d+/g);
  const words = str.match(/[a-zA-Z]+/g);

  if (words && digits && shortDurations.includes(words[0])) {
    return {[words[0]]: digits[0]};
  }
};

export const getTimeperiodForDuration = (dur: string, date?: Date): TimeParams => {
  const n = (date || new Date()).valueOf() / 1000;

  const durItems = dur.trim().split(" ");

  const durObject = durItems.reduce((prev, curr) => {

    const dur = isSupportedDuration(curr);
    if (dur) {
      return {
        ...prev,
        ...dur
      };
    } else {
      return {
        ...prev
      };
    }
  }, {});

  const delta = dayjs.duration(durObject).asSeconds();
  const step = roundToMilliseconds(delta / MAX_ITEMS_PER_CHART) || 0.001;

  return {
    start: n - delta,
    end: n,
    step: step,
    date: formatDateToUTC(date || new Date())
  };
};

export const formatDateToLocal = (date: Date): string => dayjs(date).utcOffset(0, true).local().format(dateIsoFormat);
export const formatDateToUTC = (date: Date): string => dayjs(date).utc().format(dateIsoFormat);
export const formatDateForNativeInput = (date: Date): string => dayjs(date).format(dateIsoFormat);

export const getDateNowUTC = (): Date => new Date(dayjs().utc().format(dateIsoFormat));

export const getDurationFromMilliseconds = (ms: number): string => {
  const milliseconds = Math.floor(ms  % 1000);
  const seconds = Math.floor((ms / 1000) % 60);
  const minutes = Math.floor((ms / 1000 / 60) % 60);
  const hours = Math.floor((ms / 1000 / 3600 ) % 24);
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));
  const durs: UnitTypeShort[] = ["d", "h", "m", "s", "ms"];
  const values = [days, hours, minutes, seconds, milliseconds].map((t, i) => t ? `${t}${durs[i]}` : "");
  return values.filter(t => t).join(" ");
};

export const getDurationFromPeriod = (p: TimePeriod): string => {
  const ms = p.to.valueOf() - p.from.valueOf();
  return getDurationFromMilliseconds(ms);
};

export const checkDurationLimit = (dur: string): string => {
  const durItems = dur.trim().split(" ");

  const durObject = durItems.reduce((prev, curr) => {
    const dur = isSupportedDuration(curr);
    return dur ? {...prev, ...dur} : {...prev};
  }, {});

  const delta = dayjs.duration(durObject).asMilliseconds();

  if (delta < limitsDurations.min) return getDurationFromMilliseconds(limitsDurations.min);
  if (delta > limitsDurations.max) return getDurationFromMilliseconds(limitsDurations.max);
  return dur;
};

export const dateFromSeconds = (epochTimeInSeconds: number): Date => new Date(epochTimeInSeconds * 1000);

export const relativeTimeOptions: RelativeTimeOption[] = [
  {title: "Last 5 minutes", duration: "5m"},
  {title: "Last 15 minutes", duration: "15m"},
  {title: "Last 30 minutes", duration: "30m", isDefault: true},
  {title: "Last 1 hour", duration: "1h"},
  {title: "Last 3 hours", duration: "3h"},
  {title: "Last 6 hours", duration: "6h"},
  {title: "Last 12 hours", duration: "12h"},
  {title: "Last 24 hours", duration: "24h"},
  {title: "Last 2 days", duration: "2d"},
  {title: "Last 7 days", duration: "7d"},
  {title: "Last 30 days", duration: "30d"},
  {title: "Last 90 days", duration: "90d"},
  {title: "Last 180 days", duration: "180d"},
  {title: "Last 1 year", duration: "1y"},
  {title: "Yesterday", duration: "1d", until: () => dayjs().subtract(1, "day").endOf("day").toDate()},
  {title: "Today", duration: "1d", until: () => dayjs().endOf("day").toDate()},
].map(o => ({
  id: o.title.replace(/\s/g, "_").toLocaleLowerCase(),
  until: o.until ? o.until : () => dayjs().toDate(),
  ...o
}));

export const getRelativeTime = ({relativeTimeId, defaultDuration, defaultEndInput}:
                                  { relativeTimeId?: string, defaultDuration: string, defaultEndInput: Date }) => {
  const defaultId = relativeTimeOptions.find(t => t.isDefault)?.id;
  const id = relativeTimeId || getQueryStringValue("g0.relative_time", defaultId) as string;
  const target = relativeTimeOptions.find(d => d.id === id);
  return {
    relativeTimeId: target ? id : "none",
    duration: target ? target.duration : defaultDuration,
    endInput: target ? target.until() : defaultEndInput
  };
};
