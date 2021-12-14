import {Axis, Series} from "uplot";
import {getMaxFromArray, getMinFromArray} from "../math";
import {roundTimeSeconds} from "../time";
import {AxisRange} from "../../state/graph/reducer";
import {formatTicks} from "./helpers";

export const getAxes = (series: Series[]): Axis[] => Array.from(new Set(series.map(s => s.scale))).map(a => {
  const axis = {scale: a, show: true, font: "10px Arial", values: formatTicks};
  if (!a) return {space: 80};
  if (!(Number(a) % 2)) return {...axis, side: 1};
  return axis;
});

export const getTimeSeries = (times: number[]): number[] => {
  const allTimes = Array.from(new Set(times)).sort((a, b) => a - b);
  const step = getMinFromArray(allTimes.map((t, i) => allTimes[i + 1] - t));
  const startTime = allTimes[0] || 0;
  return new Array(allTimes.length).fill(startTime).map((d, i) => roundTimeSeconds(d + (step * i)));
};

export const getLimitsYAxis = (values: { [key: string]: number[] }): AxisRange => {
  const result: AxisRange = {};
  for (const key in values) {
    const numbers = values[key];
    const min = getMinFromArray(numbers);
    const max = getMaxFromArray(numbers);
    result[key] = [min - (min * 0.05), max + (max * 0.05)];
  }
  return result;
};