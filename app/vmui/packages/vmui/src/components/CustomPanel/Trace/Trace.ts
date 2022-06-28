import {TracingData} from "../../../api/types";

export default class Trace {
  private readonly tracing: TracingData;
  private readonly tracingChildren: Trace[];
  private readonly query: string;
  private readonly id: number;
  constructor(tracingData: TracingData, query: string) {
    this.tracing = tracingData;
    this.query = query;
    this.id = new Date().getTime();
    const arr: Trace[] = [];
    this.tracingChildren = this.recursiveMap(this.tracing.children, this.createTrace, arr);
  }

  recursiveMap(oldArray: TracingData[], callback: (tr: TracingData) => Trace, newArray: Trace[]): Trace[] {
    if (!oldArray) return [];
    //base case: check if there are any items left in the original array to process
    if (oldArray && oldArray.length <= 0){
      //if all items have been processed return the new array
      return newArray;
    } else {
      //destructure the first item from old array and put remaining in a separate array
      const [item, ...theRest] = oldArray;
      // create an array of the current new array and the result of the current item and the callback function
      const interimArray = [...newArray, callback(item)];
      // return a recursive call to to map to process the next item.
      return this.recursiveMap(theRest, callback, interimArray);
    }
  }

  createTrace(traceData: TracingData) {
    return new Trace(traceData, "");
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
