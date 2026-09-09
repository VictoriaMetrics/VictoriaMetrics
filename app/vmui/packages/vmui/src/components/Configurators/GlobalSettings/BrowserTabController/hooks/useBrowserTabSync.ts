import useEventListener from "../../../../../hooks/useEventListener";
import { useEffect, useState } from "preact/compat";
import { getFromStorage, removeFromStorage, saveToStorage } from "../../../../../utils/storage";
import { getFaviconStorageKey, updateFaviconColor } from "../../../../../utils/favicon";

const storageKey = `FAVICON_COLOR:${getFaviconStorageKey()}` as const;

const getColorFromStorage = () => {
  return getFromStorage(storageKey) as string | undefined;
};

export const useBrowserTabSync = () => {
  const [faviconColor, setFaviconColor] = useState(getColorFromStorage);

  const handleUpdateColor = () => {
    setFaviconColor(getColorFromStorage());
  };

  const changeFaviconColor = (color?: string) => {
    if (color) {
      saveToStorage(storageKey, color);
    } else {
      removeFromStorage([storageKey]);
    }
  };

  useEffect(() => {
    handleUpdateColor();
  }, []);

  useEffect(() => {
    updateFaviconColor(faviconColor);
  }, [faviconColor]);

  useEventListener("storage", handleUpdateColor);

  return {
    faviconColor,
    changeFaviconColor,
  };
};
