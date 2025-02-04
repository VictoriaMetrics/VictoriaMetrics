import { TimeParams } from "../types";
import dayjs from "dayjs";
import { LOGS_BARS_VIEW, LOGS_GROUP_BY } from "../constants/logs";
import { LogHits } from "../api/types";
import { OTHER_HITS_LABEL } from "../components/Chart/BarHitsChart/hooks/useBarHitsOptions";

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

export const convertToFieldFilter = (value: string, field = LOGS_GROUP_BY) => {
  const isKeyValue = /(.+)?=(".+")/.test(value);

  if (isKeyValue) {
    return value.replace(/=/, ": ");
  }

  return `${field}: "${value}"`;
};

export const calculateTotalHits = (hits: LogHits[]): number => {
  return hits.reduce((acc, item) => acc + (item.total || 0), 0);
};

export const sortLogHits = <T extends { label?: string }>(key: keyof T) => (a: T, b: T): number => {
  if (a.label === OTHER_HITS_LABEL) return 1;
  if (b.label === OTHER_HITS_LABEL) return -1;

  const aValue = a[key] as unknown as number;
  const bValue = b[key] as unknown as number;

  return bValue - aValue;
};
