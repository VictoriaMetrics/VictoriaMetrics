export interface MetricBase {
  group: number;
  metric: {
    [key: string]: string;
  };
}

export interface MetricResult extends MetricBase {
  values: [number, string][]
}


export interface InstantMetricResult extends MetricBase {
  value: [number, string]
}

export interface TracingData {
  message: string;
  duration_msec: number;
  children: TracingData[];
}
