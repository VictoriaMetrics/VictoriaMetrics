import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import { LegendQueryParams } from "../types";
import { WITHOUT_GROUPING } from "../../../../../constants/logs";

const urlKey = LegendQueryParams.group;

export const useLegendGroup = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const valueFromUrl = searchParams.get(urlKey) || "";
  const [groupByLabel, setGroupByLabel] = useState(valueFromUrl);

  const onChange =(value: string) => {
    if (value && value !== WITHOUT_GROUPING) {
      searchParams.set(urlKey, value);
    } else {
      searchParams.delete(urlKey);
    }
    setGroupByLabel(value);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const value = searchParams.get(urlKey);
    if (value !== groupByLabel) {
      setGroupByLabel(value || "");
    }
  }, [searchParams]);

  return {
    groupByLabel,
    onChange,
  };
};
