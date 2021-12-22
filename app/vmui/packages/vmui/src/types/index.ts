import {MetricBase} from "../api/types";

export interface TimeParams {
  start: number; // timestamp in seconds
  end: number; // timestamp in seconds
  step?: number; // seconds
  date: string; // end input date
}

export interface TimePeriod {
  from: Date;
  to: Date;
}

export interface DataValue {
  key: number; // timestamp in seconds
  value: number; // y axis value
}

export interface DataSeries extends MetricBase{
  metadata: {
    name: string;
  },
  values: DataValue[]; // sorted by key which is timestamp
}

export interface InstantDataSeries {
  metadata: string[]; // just ordered columns
  value: string;
}

export enum ErrorTypes {
  emptyServer = "Please enter Server URL",
  validServer = "Please provide a valid Server URL",
  validQuery = "Please enter a valid Query and execute it"
}