import { useMemo, useState } from "preact/compat";
import { getFromStorage, saveToStorage, StorageKeys } from "../utils/storage";
import useEventListener from "./useEventListener";
import { useCallback } from "react";

/**
 * A custom hook that synchronizes a boolean state with a value stored in localStorage.
 *
 * @param {StorageKeys} key - The key used to access the corresponding value in localStorage.
 * @returns {[boolean, function]} A tuple containing the current boolean value from localStorage and a setter function to update the value in localStorage.
 *
 * The hook listens to the "storage" event to automatically update the state when the localStorage value changes.
 */
export const useLocalStorageBoolean = (key: StorageKeys): [boolean, (value: boolean) => void] => {
  const [value, setValue] = useState(!!getFromStorage(key));

  const handleUpdateStorage = useCallback(() => {
    const newValue = !!getFromStorage(key);
    if (newValue !== value) {
      setValue(newValue);
    }
  }, [key, value]);

  const setNewValue = useCallback((newValue: boolean) => {
    saveToStorage(key, newValue);
  }, [key]);

  useEventListener("storage", handleUpdateStorage);

  return useMemo(() => [value, setNewValue], [value, setNewValue]);
};
