import uPlot, {Axis} from "uplot";
import {getColorFromString} from "../color";

export const defaultOptions = {
  height: 500,
  legend: {
    show: false
  },
  cursor: {
    drag: {
      x: false,
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
      mouseup: (): null => null,
      mousedown: (): null => null,
      click: (): null => null,
      dblclick: (): null => null,
      mouseenter: (): null => null
    }
  },
};

export const formatTicks = (u: uPlot, ticks: number[], unit = ""): string[] => {
  return ticks.map(v => `${v.toString()} ${unit}`);
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

export const getColorLine = (scale: number, label: string): string => getColorFromString(`${scale}${label}`);

export const getDashLine = (group: number): number[] => group <= 1 ? [] : [group*4, group*1.2];
