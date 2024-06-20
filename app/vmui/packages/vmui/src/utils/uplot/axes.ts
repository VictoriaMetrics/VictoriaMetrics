import uPlot, { Axis, Series } from "uplot";
import { getMaxFromArray, getMinFromArray } from "../math";
import { getSecondsFromDuration, roundToMilliseconds } from "../time";
import { AxisRange } from "../../state/graph/reducer";
import { formatTicks, getTextWidth } from "./helpers";
import { TimeParams } from "../../types";
import { getCssVariable } from "../theme";
import { AxisExtend } from "../../types";

// see https://github.com/leeoniya/uPlot/tree/master/docs#axis--grid-opts
const timeValues = [
  // tick incr      default           year                            month day                      hour  min            sec   mode
  [3600 * 24 * 365, "{YYYY}",         null,                           null, null,                    null, null,          null, 1],
  [3600 * 24 * 28,  "{MMM}",          "\n{YYYY}",                     null, null,                    null, null,          null, 1],
  [3600 * 24,       "{MM}-{DD}",      "\n{YYYY}",                     null, null,                    null, null,          null, 1],
  [3600,            "{HH}:{mm}",      "\n{YYYY}-{MM}-{DD}",           null, "\n{MM}-{DD}",           null, null,          null, 1],
  [60,              "{HH}:{mm}",      "\n{YYYY}-{MM}-{DD}",           null, "\n{MM}-{DD}",           null, null,          null, 1],
  [1,               "{HH}:{mm}:{ss}", "\n{YYYY}-{MM}-{DD}",           null, "\n{MM}-{DD} {HH}:{mm}", null, null,          null, 1],
  [0.001,           ":{ss}.{fff}",    "\n{YYYY}-{MM}-{DD} {HH}:{mm}", null, "\n{MM}-{DD} {HH}:{mm}", null, "\n{HH}:{mm}", null, 1],
];

export const getAxes = (series: Series[], unit?: string): Axis[] => Array.from(new Set(series.map(s => s.scale))).map(a => {
  const font = "10px Arial";
  const stroke = getCssVariable("color-text");
  const axis = {
    scale: a,
    show: true,
    size: sizeAxis,
    stroke,
    font,
    values: (u: uPlot, ticks: number[]) => formatTicks(u, ticks, unit)
  };
  if (!a) return { space: 80, values: timeValues, stroke, font };
  if (!(Number(a) % 2) && a !== "y") return { ...axis, side: 1 };
  return axis;
});

export const getTimeSeries = (times: number[], stepDuration: string, period: TimeParams): number[] => {
  const step = getSecondsFromDuration(stepDuration) || 1;
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
  return [min - padding, max + padding];
};

export const getLimitsYAxis = (values: { [key: string]: number[] }, buffer: boolean): AxisRange => {
  const result: AxisRange = {};
  const numbers = Object.values(values).flat();
  const key = "1";
  const min = getMinFromArray(numbers) || 0;
  const max = getMaxFromArray(numbers) || 1;
  result[key] = buffer ? getMinMaxBuffer(min, max) : [min, max];
  return result;
};

export const sizeAxis = (u: uPlot, values: string[], axisIdx: number, cycleNum: number): number => {
  const axis = u.axes[axisIdx] as AxisExtend;

  if (cycleNum > 1) return axis._size || 60;

  let axisSize = 6 + (axis?.ticks?.size || 0) + (axis.gap || 0);

  const longestVal = (values ?? []).reduce((acc, val) => val?.length > acc.length ? val : acc, "");
  if (longestVal != "") axisSize += getTextWidth(longestVal, "10px Arial");

  return Math.ceil(axisSize);
};
