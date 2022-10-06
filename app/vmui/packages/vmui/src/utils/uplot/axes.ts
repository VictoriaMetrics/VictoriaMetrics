import uPlot, {Axis, Series} from "uplot";
import {getMaxFromArray, getMinFromArray} from "../math";
import {roundToMilliseconds} from "../time";
import {AxisRange} from "../../state/graph/reducer";
import {formatTicks, sizeAxis} from "./helpers";
import {TimeParams} from "../../types";

export const getAxes = (series: Series[], unit?: string): Axis[] => Array.from(new Set(series.map(s => s.scale))).map(a => {
  const axis = {
    scale: a,
    show: true,
    size: sizeAxis,
    font: "10px Arial",
    values: (u: uPlot, ticks: number[]) => formatTicks(u, ticks, unit)
  };
  if (!a) return {space: 80};
  if (!(Number(a) % 2)) return {...axis, side: 1};
  return axis;
});

export const getTimeSeries = (times: number[], step: number, period: TimeParams): number[] => {
  const allTimes = Array.from(new Set(times)).sort((a, b) => a - b);
  let t = period.start;
  const tEnd = roundToMilliseconds(period.end + step);
  let j = 0;
  const results: number[] = [];
  while (t <= tEnd) {
    while (j < allTimes.length && allTimes[j] <= t) {
      t = allTimes[j];
      j++;
      results.push(t);
    }
    t = roundToMilliseconds(t + step);
    if (j >= allTimes.length || allTimes[j] > t) {
      results.push(t);
    }
  }
  while (results.length < 2) {
    results.push(t);
    t = roundToMilliseconds(t + step);
  }
  return results;
};

export const getMinMaxBuffer = (min: number | null, max: number | null): [number, number] => {
  if (min == null || max == null) {
    return [-1, 1];
  }
  const valueRange = Math.abs(max - min) || Math.abs(min) || 1;
  const padding = 0.02*valueRange;
  return [Math.floor(min - padding), Math.ceil(max + padding)];
};

export const getLimitsYAxis = (values: { [key: string]: number[] }): AxisRange => {
  const result: AxisRange = {};
  const numbers = Object.values(values).flat();
  const key = "1";
  const min = getMinFromArray(numbers);
  const max = getMaxFromArray(numbers);
  result[key] = getMinMaxBuffer(min, max);
  return result;
};
