import { useEffect, useMemo, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import { LegendQueryParams } from "../types";
import { LegendItemType } from "../../../../../types";

const urlKey = LegendQueryParams.hideDuplicates;

export const useHideDuplicateFields = (labels?: LegendItemType[]) => {
  const [searchParams, setSearchParams] = useSearchParams();
  const valueFromUrl = searchParams.get(urlKey) === "true";
  const [hideDuplicates, setHideDuplicates] = useState(valueFromUrl);

  const onChange = (isHide: boolean) => {
    if (isHide) {
      searchParams.set(urlKey, "true");
    } else {
      searchParams.delete(urlKey);
    }

    setHideDuplicates(isHide);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const value = searchParams.get(urlKey) === "true";
    if (value !== hideDuplicates) {
      setHideDuplicates(value);
    }
  }, [searchParams]);

  const duplicateFields = useMemo(() => {
    if (!hideDuplicates || !labels?.length || labels?.length < 2) {
      return [];
    }

    const allKeys = [...new Set(labels.flatMap(l => Object.keys(l.freeFormFields || {})))];

    return allKeys.filter(key => {
      const firstValue = labels.find(l => l.freeFormFields[key])?.freeFormFields[key];
      return labels.every(l => l.freeFormFields[key] === firstValue);
    });
  }, [labels, hideDuplicates]);

  return {
    hideDuplicates,
    onChange,
    duplicateFields
  };
};
