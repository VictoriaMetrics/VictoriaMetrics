import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import { LegendQueryParams } from "../types";

export enum LegendDisplayType {
  table = "table",
  lines = "lines"
}

const urlKey = LegendQueryParams.view;

export const useLegendView = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const valueFromUrl = searchParams.get(urlKey) as LegendDisplayType;
  const [view, setView] = useState<LegendDisplayType>(valueFromUrl || LegendDisplayType.lines);

  const onChange = (type: LegendDisplayType) => {
    if (type === LegendDisplayType.table) {
      searchParams.set(urlKey, type);
    } else {
      searchParams.delete(urlKey);
    }

    setView(type);
    setSearchParams(searchParams);
  };

  useEffect(() => {
    const value = searchParams.get(urlKey);
    if (value !== view) {
      setView(value as LegendDisplayType);
    }
  }, [searchParams]);

  return {
    view,
    isLinesView: view === LegendDisplayType.lines,
    isTableView: view === LegendDisplayType.table,
    onChange
  };
};
