import {MutableRef} from "preact/hooks";

export interface TSDBStatus {
  totalSeries: number;
  totalLabelValuePairs: number;
  seriesCountByFocusLabelValue: TopHeapEntry[];
  seriesCountByMetricName: TopHeapEntry[];
  seriesCountByLabelName: TopHeapEntry[];
  seriesCountByLabelValuePair: TopHeapEntry[];
  labelValueCountByLabelName: TopHeapEntry[];
}

export interface TopHeapEntry {
  name:  string;
  count: number;
}

export type QueryUpdater = {
  [key: string]: (query: string) => string,
}

export interface Tabs {
  seriesCountByFocusLabelValue: string[];
  seriesCountByMetricName: string[];
  seriesCountByLabelName: string[];
  seriesCountByLabelValuePair: string[];
  labelValueCountByLabelName: string[];
}

export interface Containers<T> {
  seriesCountByFocusLabelValue: MutableRef<T>;
  seriesCountByMetricName: MutableRef<T>;
  seriesCountByLabelName: MutableRef<T>;
  seriesCountByLabelValuePair: MutableRef<T>;
  labelValueCountByLabelName: MutableRef<T>;
}

export interface DefaultActiveTab {
  seriesCountByFocusLabelValue: number;
  seriesCountByMetricName: number;
  seriesCountByLabelName: number;
  seriesCountByLabelValuePair: number;
  labelValueCountByLabelName: number;
}
