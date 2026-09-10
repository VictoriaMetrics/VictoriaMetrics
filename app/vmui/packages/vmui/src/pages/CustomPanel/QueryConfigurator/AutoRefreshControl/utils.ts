import { getMillisecondsFromDuration } from "../../../../utils/time";
import { MAX_REFRESH_MS, MIN_REFRESH_MS } from "./constants";

export const durationToMs = (dur: string | null) => {
  return dur ? getMillisecondsFromDuration(dur) : 0;
};

export const isValidDelay = (ms: number) => {
  return ms >= MIN_REFRESH_MS && ms <= MAX_REFRESH_MS;
};
