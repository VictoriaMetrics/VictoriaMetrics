import { RelativeTimeOption, TimeParams, TimePeriod, Timezone } from "../types";
import dayjs, { UnitTypeShort } from "dayjs";
import { getQueryStringValue } from "./query-string";
import { DATE_ISO_FORMAT } from "../constants/date";
import timezones from "../constants/timezones";
import { AppType } from "../types/appType";

const MAX_ITEMS_PER_CHART = window.innerWidth / 4;
const MAX_ITEMS_PER_HISTOGRAM = window.innerWidth / 40;

export const limitsDurations = { min: 1, max: 1.578e+11 }; // min: 1 ms, max: 5 years

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
const supportedValuesOf = Intl.supportedValuesOf;
export const supportedTimezones = supportedValuesOf ? supportedValuesOf("timeZone") as string[] : timezones;

// The list of supported units could be the following -
// https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations
export const supportedDurations = [
  { long: "years", short: "y", possible: "year" },
  { long: "weeks", short: "w", possible: "week" },
  { long: "days", short: "d", possible: "day" },
  { long: "hours", short: "h", possible: "hour" },
  { long: "minutes", short: "m", possible: "min" },
  { long: "seconds", short: "s", possible: "sec" },
  { long: "milliseconds", short: "ms", possible: "millisecond" }
];

const shortDurations = supportedDurations.map(d => d.short);

export const roundToMilliseconds = (num: number): number => Math.round(num*1000)/1000;

export const humanizeSeconds = (num: number): string => {
  return getDurationFromMilliseconds(dayjs.duration(num, "seconds").asMilliseconds());
};

export const roundStep = (step: number): string => {
  let result = roundToMilliseconds(step);
  const integerStep = Math.round(step);

  if (step >= 100) {
    result = integerStep - (integerStep%10); // integer multiple of 10
  }
  if (step < 100 && step >= 10) {
    result = integerStep - (integerStep%5); // integer multiple of 5
  }
  if (step < 10 && step >= 1) {
    result = integerStep; // integer
  }
  if (step < 1 && step > 0.01) {
    result = Math.round(step * 40) / 40; // float to thousandths multiple of 5
  }
  const humanize = humanizeSeconds(result || 0.001);
  return humanize.replace(/\s/g, "");
};

export const isSupportedDuration = (str: string): Partial<Record<UnitTypeShort, string>> | undefined => {

  const digits = str.match(/\d+/g);
  const words = str.match(/[a-zA-Z]+/g);

  if (words && digits && shortDurations.includes(words[0])) {
    return { [words[0]]: digits[0] };
  }
};

export const getSecondsFromDuration = (dur: string) => {
  const shortSupportedDur = supportedDurations.map(d => d.short).join("|");
  const regexp = new RegExp(`\\d+(\\.\\d+)?[${shortSupportedDur}]+`, "g");
  const durItems = dur.match(regexp) || [];

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

  return dayjs.duration(durObject).asSeconds();
};

export const getStepFromDuration = (dur: number, histogram?: boolean): string => {
  const size = histogram ? MAX_ITEMS_PER_HISTOGRAM : MAX_ITEMS_PER_CHART;
  return roundStep(dur / size);
};

export const getTimeperiodForDuration = (dur: string, date?: Date): TimeParams => {
  const n = (date || dayjs().toDate()).valueOf() / 1000;
  const delta = getSecondsFromDuration(dur);

  return {
    start: n - delta,
    end: n,
    step: getStepFromDuration(delta),
    date: formatDateToUTC(date || dayjs().toDate())
  };
};

export const formatDateToLocal = (date: string): Date => {
  return dayjs(date).utcOffset(0, true).toDate();
};

export const formatDateToUTC = (date: Date): string => {
  return dayjs.tz(date).utc().format(DATE_ISO_FORMAT);
};

export const formatDateForNativeInput = (date: Date): string => {
  return dayjs.tz(date).format(DATE_ISO_FORMAT);
};

export const getDateNowUTC = (): string => {
  return dayjs().utc().format(DATE_ISO_FORMAT);
};

export const getDurationFromMilliseconds = (ms: number): string => {
  const milliseconds = Math.floor(ms  % 1000);
  const seconds = Math.floor((ms / 1000) % 60);
  const minutes = Math.floor((ms / 1000 / 60) % 60);
  const hours = Math.floor((ms / 1000 / 3600 ) % 24);
  const days = Math.floor(ms / (1000 * 60 * 60 * 24));
  const durs: UnitTypeShort[] = ["d", "h", "m", "s", "ms"];
  const values = [days, hours, minutes, seconds, milliseconds].map((t, i) => t ? `${t}${durs[i]}` : "");
  return values.filter(t => t).join("");
};

export const getDurationFromPeriod = (p: TimePeriod): string => {
  const ms = p.to.valueOf() - p.from.valueOf();
  return getDurationFromMilliseconds(ms);
};

export const checkDurationLimit = (dur: string): string => {
  const durItems = dur.trim().split(" ");

  const durObject = durItems.reduce((prev, curr) => {
    const dur = isSupportedDuration(curr);
    return dur ? { ...prev, ...dur } : { ...prev };
  }, {});

  const delta = dayjs.duration(durObject).asMilliseconds();

  if (delta < limitsDurations.min) return getDurationFromMilliseconds(limitsDurations.min);
  if (delta > limitsDurations.max) return getDurationFromMilliseconds(limitsDurations.max);
  return dur;
};

export const dateFromSeconds = (epochTimeInSeconds: number): Date => {
  const date = dayjs(epochTimeInSeconds * 1000);
  return date.isValid() ? date.toDate() : new Date();
};

const getYesterday = () => dayjs().tz().subtract(1, "day").endOf("day").toDate();
const getToday = () => dayjs().tz().endOf("day").toDate();

const isLogsApp = process.env.REACT_APP_TYPE === AppType.logs;
export const relativeTimeOptions: RelativeTimeOption[] = [
  { title: "Last 5 minutes", duration: "5m", isDefault: isLogsApp },
  { title: "Last 15 minutes", duration: "15m" },
  { title: "Last 30 minutes", duration: "30m", isDefault: !isLogsApp },
  { title: "Last 1 hour", duration: "1h" },
  { title: "Last 3 hours", duration: "3h" },
  { title: "Last 6 hours", duration: "6h" },
  { title: "Last 12 hours", duration: "12h" },
  { title: "Last 24 hours", duration: "24h" },
  { title: "Last 2 days", duration: "2d" },
  { title: "Last 7 days", duration: "7d" },
  { title: "Last 30 days", duration: "30d" },
  { title: "Last 90 days", duration: "90d" },
  { title: "Last 180 days", duration: "180d" },
  { title: "Last 1 year", duration: "1y" },
  { title: "Yesterday", duration: "1d", until: getYesterday },
  { title: "Today", duration: "1d", until: getToday },
].map(o => ({
  id: o.title.replace(/\s/g, "_").toLocaleLowerCase(),
  until: o.until ? o.until : () => dayjs().tz().toDate(),
  ...o
}));

interface getRelativeTimeArguments { relativeTimeId?: string, defaultDuration: string, defaultEndInput: Date }
export const getRelativeTime = ({ relativeTimeId, defaultDuration, defaultEndInput }: getRelativeTimeArguments) => {
  const defaultId = relativeTimeOptions.find(t => t.isDefault)?.id;
  const id = relativeTimeId || getQueryStringValue("g0.relative_time", defaultId) as string;
  const target = relativeTimeOptions.find(d => d.id === id);
  return {
    relativeTimeId: target ? id : "none",
    duration: target ? target.duration : defaultDuration,
    endInput: target ? target.until() : defaultEndInput
  };
};

export const getUTCByTimezone = (timezone: string) => {
  const date = dayjs().tz(timezone);
  return `UTC${date.format("Z")}`;
};

export const getTimezoneList = (search = "") => {
  const regexp = new RegExp(search, "i");

  return supportedTimezones.reduce((acc: {[key: string]: Timezone[]}, region) => {
    const zone = (region.match(/^(.*?)\//) || [])[1] || "unknown";
    const utc = getUTCByTimezone(region);
    const utcForSearch = utc.replace(/UTC|0/, "");
    const regionForSearch = region.replace(/[/_]/g, " ");
    const item = {
      region,
      utc,
      search: `${region} ${utc} ${regionForSearch} ${utcForSearch}`
    };
    const includeZone = !search || (search && regexp.test(item.search));

    if (includeZone && acc[zone]) {
      acc[zone].push(item);
    } else if (includeZone) {
      acc[zone] = [item];
    }

    return acc;
  }, {});
};

export const setTimezone = (timezone: string) => {
  dayjs.tz.setDefault(timezone);
};

const isValidTimezone = (timezone: string) => {
  try {
    dayjs().tz(timezone);
    return true;
  } catch (e) {
    return false;
  }
};

export const getBrowserTimezone = () => {
  const timezone = dayjs.tz.guess();
  const isValid = isValidTimezone(timezone);
  return  {
    isValid,
    title: isValid ? `Browser Time (${timezone})` : "Browser timezone (UTC)",
    region: isValid ? timezone : "UTC",
  };
};
