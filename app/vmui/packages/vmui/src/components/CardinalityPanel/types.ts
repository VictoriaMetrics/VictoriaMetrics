import {MutableRef} from "preact/hooks";

export interface TSDBStatus {
  totalSeries: number;
  totalLabelValuePairs: number;
  seriesCountByMetricName: TopHeapEntry[];
  seriesCountByLabelName: TopHeapEntry[];
  seriesCountByFocusLabelValue: TopHeapEntry[];
  seriesCountByLabelValuePair: TopHeapEntry[];
  labelValueCountByLabelName: TopHeapEntry[];
}

export interface TopHeapEntry {
  name:  string;
  count: number;
}

export type QueryUpdater = {
  [key: string]: (focusLabel: string | null, query: string) => string,
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

export interface DefaultActiveTab {
  seriesCountByMetricName: number;
  seriesCountByLabelName: number;
  seriesCountByFocusLabelValue: number;
  seriesCountByLabelValuePair: number;
  labelValueCountByLabelName: number;
}
