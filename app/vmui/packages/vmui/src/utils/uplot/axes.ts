import {Axis, Series} from "uplot";
import {getMaxFromArray, getMinFromArray} from "../math";
import {roundTimeSeconds} from "../time";
import {AxisRange} from "../../state/graph/reducer";
import {formatTicks} from "./helpers";
import {TimeParams} from "../../types";

export const getAxes = (series: Series[]): Axis[] => Array.from(new Set(series.map(s => s.scale))).map(a => {
  const axis = {scale: a, show: true, font: "10px Arial", values: formatTicks};
  if (!a) return {space: 80};
  if (!(Number(a) % 2)) return {...axis, side: 1};
  return axis;
});

export const getTimeSeries = (times: number[], defaultStep: number, period: TimeParams): number[] => {
  const allTimes = Array.from(new Set(times)).sort((a, b) => a - b);
  const length = Math.ceil((period.end - period.start)/defaultStep);
  const startTime = allTimes[0] || 0;
  return new Array(length*2).fill(startTime).map((d, i) => roundTimeSeconds(d + (defaultStep * i)));
};

export const getMinMaxBuffer = (min: number, max: number): [number, number] => {
  const minCorrect = isNaN(min) ? -1 : min;
  const maxCorrect = isNaN(max) ? 1 : max;
  const valueRange = Math.abs(maxCorrect - minCorrect) || Math.abs(minCorrect) || 1;
  const padding = 0.02*valueRange;
  return [minCorrect - padding, maxCorrect + padding];
};

export const getLimitsYAxis = (values: { [key: string]: number[] }): AxisRange => {
  const result: AxisRange = {};
  for (const key in values) {
    const numbers = values[key];
    const min = getMinFromArray(numbers);
    const max = getMaxFromArray(numbers);
    result[key] = getMinMaxBuffer(min, max);
  }
  return result;
};
