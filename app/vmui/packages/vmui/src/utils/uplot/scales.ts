import uPlot, { Range, Scale, Scales } from "uplot";
import { getMinMaxBuffer } from "./axes";
import { YaxisState } from "../../state/graph/reducer";
import { MinMax, SetMinMax } from "../../types";

export const getRangeX = ({ min, max }: MinMax): Range.MinMax => [min, max];

export const getRangeY = (u: uPlot, min = 0, max = 1, axis: string, yaxis: YaxisState): Range.MinMax => {
  if (yaxis.limits.enable) return yaxis.limits.range[axis];
  return getMinMaxBuffer(min, max);
};

export const getScales = (yaxis: YaxisState, xRange: MinMax): Scales => {
  const scales: { [key: string]: { range: Scale.Range } } = { x: { range: () => getRangeX(xRange) } };
  const ranges = Object.keys(yaxis.limits.range);
  (ranges.length ? ranges : ["1"]).forEach(axis => {
    scales[axis] = { range: (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis, yaxis) };
  });
  return scales;
};

export const setSelect = (setPlotScale: SetMinMax) => (u: uPlot) => {
  const min = u.posToVal(u.select.left, "x");
  const max = u.posToVal(u.select.left + u.select.width, "x");
  setPlotScale({ min, max });
};
