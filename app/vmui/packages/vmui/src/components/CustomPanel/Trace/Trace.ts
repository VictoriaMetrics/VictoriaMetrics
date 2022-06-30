import {TracingData} from "../../../api/types";

let traceId = 0;

export default class Trace {
  private readonly tracing: TracingData;
  private readonly tracingChildren: Trace[];
  private readonly query: string;
  private readonly id: number;
  constructor(tracingData: TracingData, query: string) {
    this.tracing = tracingData;
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
}
