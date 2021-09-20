import {TimeParams, TimePeriod} from "../types";

import dayjs, {UnitTypeShort} from "dayjs";
import duration from "dayjs/plugin/duration";

dayjs.extend(duration);

const MAX_ITEMS_PER_CHART = window.screen.availWidth / 2;

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
  const step = Math.ceil(delta / MAX_ITEMS_PER_CHART);

  return {
    start: n - delta,
    end: n,
    step: step,
    date: formatDateForNativeInput((date || new Date()))
  };
};

export const formatDateForNativeInput = (date: Date): string => dayjs(date).format("YYYY-MM-DD[T]HH:mm:ss");

export const getDurationFromPeriod = (p: TimePeriod): string => {
  const ms = p.to.valueOf() - p.from.valueOf();
  const seconds = Math.floor((ms / 1000) % 60);
  const minutes = Math.floor((ms / 1000 / 60) % 60);
  const hours = Math.floor((ms / 1000 / 3600 ) % 24);
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));
  const durs: UnitTypeShort[] = ["d", "h", "m", "s"];
  const values = [days, hours, minutes, seconds].map((t, i) => t ? `${t}${durs[i]}` : "");
  return values.filter(t => t).join(" ");
};

export const dateFromSeconds = (epochTimeInSeconds: number): Date =>
  new Date(epochTimeInSeconds * 1000);
