import { QueryContextType } from "../types";

export const AUTOCOMPLETE_LIMITS = {
  displayResults: 50,
  queryLimit: 1000,
  cacheLimit: 1000,
};

export const AUTOCOMPLETE_MIN_SYMBOLS = {
  [QueryContextType.metricsql]: 2,
  [QueryContextType.empty]: 2,
  [QueryContextType.label]: 0,
  [QueryContextType.labelValue]: 0,
};
