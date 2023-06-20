import uPlot, { Range, Scale, Scales } from "uplot";
import { getMinMaxBuffer } from "./axes";
import { YaxisState } from "../../state/graph/reducer";

interface XRangeType {
  min: number,
  max: number
}

export const getRangeX = (xRange: XRangeType): Range.MinMax => {
  return [xRange.min, xRange.max];
};

export const getRangeY = (u: uPlot, min = 0, max = 1, axis: string, yaxis: YaxisState): Range.MinMax => {
  if (yaxis.limits.enable) return yaxis.limits.range[axis];
  return getMinMaxBuffer(min, max);
};

export const getScales = (yaxis: YaxisState, xRange: XRangeType): Scales => {
  const scales: { [key: string]: { range: Scale.Range } } = { x: { range: () => getRangeX(xRange) } };
  const ranges = Object.keys(yaxis.limits.range);
  (ranges.length ? ranges : ["1"]).forEach(axis => {
    scales[axis] = { range: (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis, yaxis) };
  });
  return scales;
};
