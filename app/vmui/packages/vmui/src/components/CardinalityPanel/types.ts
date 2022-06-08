import {MutableRef} from "preact/hooks";

export interface TSDBStatus {
  labelValueCountByLabelName: TopHeapEntry[];
  seriesCountByLabelValuePair: TopHeapEntry[];
  seriesCountByMetricName: TopHeapEntry[];
  totalSeries: number;
  totalLabelValuePairs: number;
}

export interface TopHeapEntry {
  name:  string;
  count: number;
}

export type TypographyFunctions = {
  [key: string]: (value: number) => string,
}

export type QueryUpdater = {
  [key: string]: (query: string) => string,
}

export interface Tabs {
  labelValueCountByLabelName: string[];
  seriesCountByLabelValuePair: string[];
  seriesCountByMetricName: string[];
}

export interface Containers<T> {
  labelValueCountByLabelName: MutableRef<T>;
  seriesCountByLabelValuePair: MutableRef<T>;
  seriesCountByMetricName: MutableRef<T>;
}

export interface DefaultState {
  labelValueCountByLabelName: number;
  seriesCountByLabelValuePair: number;
  seriesCountByMetricName: number;
}
