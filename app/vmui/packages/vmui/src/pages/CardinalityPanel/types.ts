import { MutableRef } from "preact/hooks";

export interface TSDBStatus {
  totalSeries: number;
  totalLabelValuePairs: number;
  totalSeriesByAll: number,
  totalSeriesPrev: number,
  seriesCountByMetricName: TopHeapEntry[];
  seriesCountByLabelName: TopHeapEntry[];
  seriesCountByFocusLabelValue: TopHeapEntry[];
  seriesCountByLabelValuePair: TopHeapEntry[];
  labelValueCountByLabelName: TopHeapEntry[];
  headStats?: object;
}

export interface TopHeapEntry {
  name:  string;
  value: number;
  diff: number;
  valuePrev: number;
}

interface QueryUpdaterArgs {
  query: string;
  focusLabel: string;
  match: string;
}

export type QueryUpdater = {
  [key: string]: (args: QueryUpdaterArgs) => string,
}

export interface Tabs {
  seriesCountByMetricName: string[];
  seriesCountByLabelName: string[];
  seriesCountByFocusLabelValue: string[];
  seriesCountByLabelValuePair: string[];
  labelValueCountByLabelName: string[];
}

export interface Containers<T> {
  seriesCountByMetricName: MutableRef<T>;
  seriesCountByLabelName: MutableRef<T>;
  seriesCountByFocusLabelValue: MutableRef<T>;
  seriesCountByLabelValuePair: MutableRef<T>;
  labelValueCountByLabelName: MutableRef<T>;
}
