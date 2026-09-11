import { REFRESH_DEFAULT_VALUE, REFRESH_URL_PARAM } from "../constants";
import { durationToMs, isValidDelay } from "../utils";
import { useSearchParams } from "react-router";

export const useAutoRefreshInterval = () => {
  const [searchParams, setSearchParams] = useSearchParams();

  const rawRefreshInterval = searchParams.get(REFRESH_URL_PARAM);
  const msRefresh = durationToMs(rawRefreshInterval);
  const refreshInterval = rawRefreshInterval && isValidDelay(msRefresh) ? rawRefreshInterval : REFRESH_DEFAULT_VALUE;

  const setRefreshInterval = (duration: string) => {
    const ms = durationToMs(duration);

    setSearchParams(prev => {
      const nextParams = new URLSearchParams(prev);

      if (isValidDelay(ms)) {
        nextParams.set(REFRESH_URL_PARAM, duration);
      } else {
        nextParams.delete(REFRESH_URL_PARAM);
      }

      return nextParams;
    });
  };

  return {
    refreshInterval,
    setRefreshInterval,
  };
};
