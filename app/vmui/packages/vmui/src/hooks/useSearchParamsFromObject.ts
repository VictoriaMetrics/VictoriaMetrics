import { useNavigate, useSearchParams } from "react-router-dom";
import { useCallback } from "preact/compat";


const useSearchParamsFromObject = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const setSearchParamsFromKeys = useCallback((objectParams: Record<string, string | number>) => {
    const hasSearchParams = !!searchParams.size;
    let hasChanged = false;

    const newSearchParams = new URLSearchParams(searchParams);
    searchParams.keys().forEach(key => {
      if (!(key in objectParams)) {
        newSearchParams.delete(key);
        hasChanged = true;
      }
    });

    Object.entries(objectParams).forEach(([key, value]) => {
      if (newSearchParams.get(key) !== `${value}`) {
        newSearchParams.set(key, `${value}`);
        hasChanged = true;
      }
    });

    if (!hasChanged) return;

    if (hasSearchParams) {
      setSearchParams(newSearchParams);
    } else {
      navigate(`?${newSearchParams.toString()}`, { replace: true });
    }
  }, [searchParams, navigate]);

  return {
    setSearchParamsFromKeys
  };
};

export default useSearchParamsFromObject;
