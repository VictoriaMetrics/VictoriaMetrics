import { getMillisecondsFromDuration } from "../../../../utils/time";

export const REFRESH_URL_PARAM = "refresh";

export const REFRESH_DISABLE_OPTION = "Off";

export const REFRESH_OPTIONS = [
  REFRESH_DISABLE_OPTION,
  "1s",
  "2s",
  "5s",
  "10s",
  "30s",
  "1m",
  "5m",
  "15m",
  "30m",
  "1h",
  "2h"
];

export const REFRESH_DEFAULT_VALUE = REFRESH_OPTIONS[0];

export const MIN_REFRESH_MS = 1000;
export const MAX_REFRESH_MS = getMillisecondsFromDuration(REFRESH_OPTIONS[REFRESH_OPTIONS.length - 1]);
