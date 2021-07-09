import {TimeParams, TimePeriod} from "../types";

import dayjs, {UnitTypeShort} from "dayjs";
import duration from "dayjs/plugin/duration";

dayjs.extend(duration);

const MAX_ITEMS_PER_CHART = 30; // TODO: make dependent from screen size

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

  return {
    start: n - delta,
    end: n,
    step: delta / MAX_ITEMS_PER_CHART
  };
};

export const formatDateForNativeInput = (date: Date): string => {
  const isoString = dayjs(date).format("YYYY-MM-DD[T]HH:mm:ss");
  return isoString;
};

export const getDurationFromPeriod = (p: TimePeriod): string => {
  const dur = dayjs.duration(p.to.valueOf() - p.from.valueOf());
  const durs: UnitTypeShort[] = ["d", "h", "m", "s"];
  return durs
    .map(d => ({val: dur.get(d), str: d}))
    .filter(obj => obj.val !== 0)
    .map(obj => `${obj.val}${obj.str}`)
    .join(" ");
};

export const dateFromSeconds = (epochTimeInSeconds: number): Date =>
  new Date(epochTimeInSeconds * 1000);
