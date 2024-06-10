import uPlot, { Range, Scale, Scales } from "uplot";
import { getMinMaxBuffer } from "./axes";
import { YaxisState } from "../../state/graph/reducer";
import { ForecastType, MinMax, SetMinMax } from "../../types";
import { anomalyColors } from "../color";

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

export const scaleGradient = (
  scaleKey: string,
  ori: number,
  scaleStops: [number, string][],
  discrete = false
) => (u: uPlot): CanvasGradient | string => {
  const can = document.createElement("canvas");
  const ctx = can.getContext("2d");
  if (!ctx) return "";

  const scale = u.scales[scaleKey];

  // we want the stop below or at the scaleMax
  // and the stop below or at the scaleMin, else the stop above scaleMin
  let minStopIdx = 0;
  let maxStopIdx = 1;

  for (let i = 0; i < scaleStops.length; i++) {
    const stopVal = scaleStops[i][0];

    if (stopVal <= (scale.min || 0) || minStopIdx == null)
      minStopIdx = i;

    maxStopIdx = i;

    if (stopVal >= (scale.max || 1))
      break;
  }

  if (minStopIdx == maxStopIdx)
    return scaleStops[minStopIdx][1];

  let minStopVal = scaleStops[minStopIdx][0];
  let maxStopVal = scaleStops[maxStopIdx][0];

  if (minStopVal == -Infinity)
    minStopVal = scale.min || 0;

  if (maxStopVal == Infinity)
    maxStopVal = scale.max || 1;

  const minStopPos = u.valToPos(minStopVal, scaleKey, true) || 0;
  const maxStopPos = u.valToPos(maxStopVal, scaleKey, true) || 1;

  const range = minStopPos - maxStopPos;

  let x0, y0, x1, y1;

  if (ori == 1) {
    x0 = x1 = 0;
    y0 = minStopPos;
    y1 = maxStopPos;
  } else {
    y0 = y1 = 0;
    x0 = minStopPos;
    x1 = maxStopPos;
  }

  const grd = ctx.createLinearGradient(x0, y0, x1, y1);

  let prevColor = anomalyColors[ForecastType.actual];

  for (let i = minStopIdx; i <= maxStopIdx; i++) {
    const s = scaleStops[i];

    const stopPos = i == minStopIdx ? minStopPos : i == maxStopIdx ? maxStopPos : u.valToPos(s[0], scaleKey, true) | 1;
    const pct = Math.min(1, Math.max(0, (minStopPos - stopPos) / range));
    if (discrete && i > minStopIdx) {
      grd.addColorStop(pct, prevColor);
    }

    grd.addColorStop(pct, prevColor = s[1]);
  }

  return grd;
};
