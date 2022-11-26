import { TracingData } from "../../api/types";

let traceId = 0;

export default class Trace {
  private tracing: TracingData;
  private query: string;
  private tracingChildren: Trace[];
  private readonly originalTracing: TracingData;
  private readonly id: number;
  constructor(tracingData: TracingData, query: string) {
    this.tracing = tracingData;
    this.originalTracing = JSON.parse(JSON.stringify(tracingData));
    this.query = query;
    this.id = traceId++;
    const children = tracingData.children || [];
    this.tracingChildren = children.map((x: TracingData) => new Trace(x, query));
  }

  get queryValue(): string {
    return this.query;
  }
  get idValue(): number {
    return this.id;
  }
  get children(): Trace[] {
    return this.tracingChildren;
  }
  get message(): string {
    return this.tracing.message;
  }
  get duration(): number {
    return this.tracing.duration_msec;
  }
  get JSON(): string {
    return JSON.stringify(this.tracing, null, 2);
  }
  get originalJSON(): string {
    return JSON.stringify(this.originalTracing, null, 2);
  }
  setTracing (tracingData: TracingData) {
    this.tracing = tracingData;
    const children = tracingData.children || [];
    this.tracingChildren = children.map((x: TracingData) => new Trace(x, this.query));
  }
  setQuery (query: string) {
    this.query = query;
  }
  resetTracing () {
    this.tracing = this.originalTracing;
  }
}
