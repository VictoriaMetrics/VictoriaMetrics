import { TimeParams } from "../types";
import dayjs from "dayjs";
import { LOGS_BARS_VIEW } from "../constants/logs";

export const getStreamPairs = (value: string): string[] => {
  const pairs = /^{.+}$/.test(value) ? value.slice(1, -1).split(",") : [value];
  return pairs.filter(Boolean);
};

export const getHitsTimeParams = (period: TimeParams) => {
  const start = dayjs(period.start * 1000);
  const end = dayjs(period.end * 1000);
  const totalSeconds = end.diff(start, "milliseconds");
  const step = Math.ceil(totalSeconds / LOGS_BARS_VIEW) || 1;
  return { start, end, step };
};
