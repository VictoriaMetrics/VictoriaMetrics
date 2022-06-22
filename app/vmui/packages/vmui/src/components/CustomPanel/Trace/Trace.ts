import {TracingData} from "../../../api/types";

export default class Trace {
  private tracing: TracingData;
  private query: string;
  private id: number;
  constructor(tracingData: TracingData, query: string) {
    this.tracing = tracingData;
    this.query = query;
    this.id = new Date().getTime();
  }

  get queryValue(): string {
    return this.query;
  }
  get idValue(): number {
    return this.id;
  }
  get tracingValue(): TracingData {
    return this.tracing;
  }
}
