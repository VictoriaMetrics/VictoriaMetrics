import { useMemo } from "preact/compat";
import { MetricBase } from "../api/types";

export type MetricCategory = {
  key: string;
  variations: number;
}

export const getColumns = (data: MetricBase[]): MetricCategory[] => {
  const columns: { [key: string]: { options: Set<string> } } = {};
  data.forEach(d =>
    Object.entries(d.metric).forEach(e =>
      columns[e[0]] ? columns[e[0]].options.add(e[1]) : columns[e[0]] = { options: new Set([e[1]]) }
    )
  );

  return Object.entries(columns).map(e => ({
    key: e[0],
    variations: e[1].options.size
  })).sort((a1, a2) => a1.variations - a2.variations);
};

export const useSortedCategories = (data: MetricBase[], displayColumns?: string[]): MetricCategory[] => (
  useMemo(() => {
    const sortedColumns = getColumns(data);
    return displayColumns ? sortedColumns.filter(col => displayColumns.includes(col.key)) : sortedColumns;
  }, [data, displayColumns])
);
