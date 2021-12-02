export type StorageKeys = "BASIC_AUTH_DATA"
    | "BEARER_AUTH_DATA"
    | "AUTH_TYPE"
    | "AUTOCOMPLETE"
    | "NO_CACHE"

export const saveToStorage = (key: StorageKeys, value: string | boolean | Record<string, unknown>): void => {
  if (value) {
    // keeping object in storage so that keeping the string is not different from keeping
    window.localStorage.setItem(key, JSON.stringify({value}));
  } else {
    removeFromStorage([key]);
  }
};

// TODO: make this aware of data type that is stored
export const getFromStorage = (key: StorageKeys): undefined | boolean | string | Record<string, unknown>  => {
  const valueObj = window.localStorage.getItem(key);
  if (valueObj === null) {
    return undefined;
  } else {
    try {
      return JSON.parse(valueObj)?.value; // see comment in "saveToStorage"
    } catch (e) {
      return valueObj; // fallback for corrupted json
    }
  }
};

export const removeFromStorage = (keys: StorageKeys[]): void => keys.forEach(k => window.localStorage.removeItem(k));

export const authKeys: StorageKeys[] = ["BASIC_AUTH_DATA", "BEARER_AUTH_DATA"];

