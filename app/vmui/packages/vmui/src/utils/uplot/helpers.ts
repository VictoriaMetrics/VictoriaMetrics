import uPlot, { Axis } from "uplot";
import { getColorFromString } from "../color";

export const defaultOptions = {
  legend: {
    show: false
  },
  cursor: {
    drag: {
      x: true,
      y: false
    },
    focus: {
      prox: 30
    },
    points: {
      size: 5.6,
      width: 1.4
    },
    bind: {
      click: (): null => null,
      dblclick: (): null => null,
    },
  },
};

export const formatTicks = (u: uPlot, ticks: number[], unit = ""): string[] => {
  const min = ticks[0];
  const max = ticks[ticks.length-1];
  if (!unit) {
    return ticks.map(v => formatPrettyNumber(v, min, max));
  }
  return ticks.map(v => `${formatPrettyNumber(v, min, max)} ${unit}`);
};

export const formatPrettyNumber = (n: number | null | undefined, min = 0, max = 0): string => {
  if (n === undefined || n === null) {
    return "";
  }
  const range = Math.abs(max - min);
  if (isNaN(range) || range == 0) {
    // Return the constant number as is if the range isn't set of it is too small.
    if (Math.abs(n) >= 1000) {
      return n.toLocaleString("en-US");
    }
    return n.toString();
  }
  // Make sure n has 3 significant digits on the given range.
  // This precision should be enough for most UX cases,
  // since the remaining digits are usually a white noise.
  let digits = 3 + Math.floor(1 + Math.log10(Math.max(Math.abs(min), Math.abs(max))) - Math.log10(range));
  if (isNaN(digits) || digits > 20) {
    digits = 20;
  }
  return n.toLocaleString("en-US", {
    minimumSignificantDigits: digits,
    maximumSignificantDigits: digits,
  });
};

interface AxisExtend extends Axis {
  _size?: number;
}

const getTextWidth = (val: string, font: string): number => {
  const span = document.createElement("span");
  span.innerText = val;
  span.style.cssText = `position: absolute; z-index: -1; pointer-events: none; opacity: 0; font: ${font}`;
  document.body.appendChild(span);
  const width = span.offsetWidth;
  span.remove();
  return width;
};

export const sizeAxis = (u: uPlot, values: string[], axisIdx: number, cycleNum: number): number => {
  const axis = u.axes[axisIdx] as AxisExtend;

  if (cycleNum > 1) return axis._size || 60;

  let axisSize = 6 + (axis?.ticks?.size || 0) + (axis.gap || 0);

  const longestVal = (values ?? []).reduce((acc, val) => val.length > acc.length ? val : acc, "");
  if (longestVal != "") axisSize += getTextWidth(longestVal, u.ctx.font);

  return Math.ceil(axisSize);
};

export const getColorLine = (label: string): string => getColorFromString(label);

export const getDashLine = (group: number): number[] => group <= 1 ? [] : [group*4, group*1.2];
