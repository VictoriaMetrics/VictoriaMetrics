const STORAGE_PREFIX = "VMUI:" as const;

/**
 * Do not use this type in local storage type
 * @deprecated
 * */
const DEPRECATED_STORAGE_KEYS = ["QUERY_HISTORY", "QUERY_FAVORITES"] as const;
export type DeprecatedStorageKeys = (typeof DEPRECATED_STORAGE_KEYS)[number];

export const ALL_STORAGE_KEYS = [
  "AUTOCOMPLETE",
  "NO_CACHE",
  "QUERY_TRACING",
  "SERIES_LIMITS",
  "LEGEND_AUTO_COLLAPSE",
  "TABLE_COMPACT",
  "TIMEZONE",
  "DISABLED_DEFAULT_TIMEZONE",
  "THEME",
  "EXPLORE_METRICS_TIPS",
  "METRICS_QUERY_HISTORY",
  "SERVER_URL",
  "POINTS_SHOW_ALL",
  ...DEPRECATED_STORAGE_KEYS,
] as const;

type StorageKeys = (typeof ALL_STORAGE_KEYS)[number];
export type ActiveStorageKeys = Exclude<StorageKeys, DeprecatedStorageKeys>;

type PrefixedStorageKeys = `${typeof STORAGE_PREFIX}${StorageKeys}`;

const toPrefixedKey = (key: ActiveStorageKeys): PrefixedStorageKeys => {
  return `${STORAGE_PREFIX}${key}`;
};

type StorageValue = string | boolean | Record<string, unknown>;

export const saveToStorage = (key: ActiveStorageKeys, value: StorageValue, withPrefix = true): void => {
  try {
    const storageKey = withPrefix ? toPrefixedKey(key) : key;

    if (value) {
      // keeping object in storage so that keeping the string is not different from keeping
      window.localStorage.setItem(storageKey, JSON.stringify({ value }));
    } else {
      window.localStorage.removeItem(storageKey);
    }
    window.dispatchEvent(new Event("storage"));
  } catch (e) {
    console.error(e);
  }
};

export const getFromStorage = (key: ActiveStorageKeys, withPrefix = true): undefined | StorageValue => {
  const storageKey = withPrefix ? toPrefixedKey(key) : key;
  const valueObj = window.localStorage.getItem(storageKey);

  if (valueObj === null) return undefined;

  try {
    return JSON.parse(valueObj)?.value; // see comment in "saveToStorage"
  } catch (e) {
    return valueObj; // fallback for corrupted json
  }
};

export const removeFromStorage = (keys: ActiveStorageKeys[], withPrefix = true): void => {
  const storageKeys = withPrefix ? keys.map(toPrefixedKey) : keys;
  storageKeys.forEach(k => window.localStorage.removeItem(k));
};

/**
 * Migrates legacy (unprefixed) localStorage keys to the new prefixed format (`${STORAGE_PREFIX}*`).
 * Keeps the prefixed value if it already exists, then removes the legacy key.
 */

type StorageMigrationResult = {
  migrated: ActiveStorageKeys[];
  removed: ActiveStorageKeys[];
  skipped: ActiveStorageKeys[];
};

export const migrateStorageToPrefixedKeys = (): StorageMigrationResult => {
  const res: StorageMigrationResult = {
    migrated: [],
    removed: [],
    skipped: [],
  };

  for (const key of ALL_STORAGE_KEYS) {
    if (DEPRECATED_STORAGE_KEYS.includes(key as DeprecatedStorageKeys)) continue;

    const legacyKey = key as ActiveStorageKeys; // unprefixed
    const legacyValue = getFromStorage(legacyKey, false);
    const prefixedValue = getFromStorage(legacyKey, true);

    if (legacyValue === undefined) {
      res.skipped.push(legacyKey);
      continue;
    }

    // prefixed exists -> keep it, just remove legacy
    if (prefixedValue !== undefined) {
      removeFromStorage([legacyKey], false);
      res.removed.push(legacyKey);
      continue;
    }

    // prefixed missing -> copy legacy -> prefixed, then remove legacy
    saveToStorage(legacyKey, legacyValue, true);
    removeFromStorage([legacyKey], false);
    res.migrated.push(legacyKey);
  }

  return res;
};
