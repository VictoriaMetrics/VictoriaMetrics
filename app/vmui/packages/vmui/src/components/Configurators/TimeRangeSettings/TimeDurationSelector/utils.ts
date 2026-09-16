import { normalizeDuration } from "../../../../utils/time";
import { getFromStorage, saveToStorage } from "../../../../utils/storage";

const maxRecentDurations = 3;
const storageKey = "CUSTOM_TIME_RANGES" as const;

interface StoredCustomTimeRanges {
  durations?: unknown;
}

export const normalizeRecentDurations = (durations: unknown): string[] => {
  if (!Array.isArray(durations)) return [];

  const result: string[] = [];
  for (const value of durations) {
    if (typeof value !== "string") continue;
    const duration = normalizeDuration(value);
    if (!duration || result.includes(duration)) continue;
    result.push(duration);
    if (result.length === maxRecentDurations) break;
  }
  return result;
};

export const addRecentDuration = (durations: string[], value: string): string[] => {
  const duration = normalizeDuration(value);
  if (!duration) return normalizeRecentDurations(durations);
  return [duration, ...normalizeRecentDurations(durations).filter(item => item !== duration)]
    .slice(0, maxRecentDurations);
};

export const getRecentDurations = (): string[] => {
  const stored = getFromStorage(storageKey) as StoredCustomTimeRanges | undefined;
  return normalizeRecentDurations(stored?.durations);
};

export const saveRecentDuration = (value: string): string[] => {
  const durations = addRecentDuration(getRecentDurations(), value);
  saveToStorage(storageKey, { durations });
  return durations;
};
