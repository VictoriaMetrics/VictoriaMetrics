import { AUTOCOMPLETE_LIMITS } from "../../../constants/queryAutocomplete";

export type QueryAutocompleteCacheItem = {
  type: string;
  value: string;
  start: string;
  end: string;
  match: string;
}

export class QueryAutocompleteCache {
  private maxSize: number;
  private map: Map<string, string[]>;

  constructor() {
    this.maxSize = AUTOCOMPLETE_LIMITS.cacheLimit;
    this.map = new Map();
  }

  get(key: QueryAutocompleteCacheItem) {
    for (const [cacheKey, cacheValue] of this.map) {
      const cacheItem = JSON.parse(cacheKey) as QueryAutocompleteCacheItem;

      const equalRange = cacheItem.start === key.start && cacheItem.end === key.end;
      const equalType = cacheItem.type === key.type;
      const isIncluded = key.value && cacheItem.value && key.value.includes(cacheItem.value);
      const isSimilar = (cacheItem.match === key.match) || isIncluded;
      const isUnderLimit = cacheValue.length < AUTOCOMPLETE_LIMITS.queryLimit;
      if (isSimilar && equalRange && equalType && isUnderLimit) {
        return cacheValue;
      }
    }
    return this.map.get(JSON.stringify(key));
  }

  put(key: QueryAutocompleteCacheItem, value: string[]) {
    if (this.map.size >= this.maxSize) {
      const firstKey = this.map.keys().next().value;
      this.map.delete(firstKey);
    }
    this.map.set(JSON.stringify(key), value);
  }
}
