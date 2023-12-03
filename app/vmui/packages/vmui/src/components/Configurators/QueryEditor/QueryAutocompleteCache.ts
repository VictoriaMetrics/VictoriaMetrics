export type QueryAutocompleteCacheItem = {
  type: string;
  value: string;
  start: string;
  end: string;
  match: string;
}

export const AUTOCOMPLETE_LIMITS = {
  displayResults: 50,
  queryLimit: 1000,
  cacheLimit: 1000,
};

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
      const isSimilar = cacheItem.match === key.match || key.value.includes(cacheItem.value);
      const limitNotReached = cacheValue.length < AUTOCOMPLETE_LIMITS.queryLimit;
      if (isSimilar && equalRange && equalType && limitNotReached) {
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
