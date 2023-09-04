import { useNavigate, useSearchParams } from "react-router-dom";
import { useCallback } from "preact/compat";


const useSearchParamsFromObject = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const setSearchParamsFromKeys = useCallback((objectParams: Record<string, string | number>) => {
    const hasSearchParams = !!Array.from(searchParams.values()).length;
    let hasChanged = false;

    Object.entries(objectParams).forEach(([key, value]) => {
      if (searchParams.get(key) !== `${value}`) {
        searchParams.set(key, `${value}`);
        hasChanged = true;
      }
    });

    if (!hasChanged) return;

    if (hasSearchParams) {
      setSearchParams(searchParams);
    } else {
      navigate(`?${searchParams.toString()}`, { replace: true });
    }
  }, [searchParams, navigate]);

  return {
    setSearchParamsFromKeys
  };
};

export default useSearchParamsFromObject;
