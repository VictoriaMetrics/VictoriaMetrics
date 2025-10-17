import { useSearchParams } from "react-router-dom";
import { useCallback } from "preact/compat";

type ParamValue = string | number | boolean | null | undefined;

const useSearchParamsFromObject = () => {
  const [searchParams, setSearchParams] = useSearchParams();

  const setSearchParamsFromKeys = useCallback((objectParams: Record<string, ParamValue>) => {
    const hadParams = !!searchParams.size;

    const newSearchParams = new URLSearchParams(searchParams);
    const beforeParams = searchParams.toString();

    for (const [key, newValue] of Object.entries(objectParams)) {
      const isEmpty = newValue === null || newValue === undefined || newValue === "";

      if (isEmpty) {
        newSearchParams.delete(key);
        continue;
      }

      const next = String(newValue);
      if (newSearchParams.get(key) !== next) {
        newSearchParams.set(key, next);
      }
    }

    if (beforeParams === newSearchParams.toString()) return;

    setSearchParams(newSearchParams, { replace: !hadParams });
  },
  [searchParams, setSearchParams]
  );

  return { setSearchParamsFromKeys };
};

export default useSearchParamsFromObject;
