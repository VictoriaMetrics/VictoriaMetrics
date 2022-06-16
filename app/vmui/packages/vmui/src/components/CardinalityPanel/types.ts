import {MutableRef} from "preact/hooks";

export interface TSDBStatus {
  totalSeries: number;
  totalLabelValuePairs: number;
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
  seriesCountByMetricName: string[];
  seriesCountByLabelName: string[];
  seriesCountByLabelValuePair: string[];
  labelValueCountByLabelName: string[];
}

export interface Containers<T> {
  seriesCountByMetricName: MutableRef<T>;
  seriesCountByLabelName: MutableRef<T>;
  seriesCountByLabelValuePair: MutableRef<T>;
  labelValueCountByLabelName: MutableRef<T>;
}

export interface DefaultState {
  seriesCountByMetricName: number;
  seriesCountByLabelName: number;
  seriesCountByLabelValuePair: number;
  labelValueCountByLabelName: number;
}
