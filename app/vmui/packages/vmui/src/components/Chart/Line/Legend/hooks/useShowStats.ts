import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import { LegendQueryParams } from "../types";

const urlKey = LegendQueryParams.hideStats;

export const useShowStats = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const valueFromUrl = searchParams.get(urlKey) === "true";
  const [hideStats, setHideStats] = useState(valueFromUrl);

  const onChange = (isHide: boolean) => {
    if (isHide) {
      searchParams.set(urlKey, "true");
    } else {
      searchParams.delete(urlKey);
    }

    setHideStats(isHide);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const value = searchParams.get(urlKey) === "true";
    if (value !== hideStats) {
      setHideStats(value);
    }
  }, [searchParams]);

  return {
    hideStats,
    onChange
  };
};
