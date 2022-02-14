import uPlot from "uplot";
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

export const formatTicks = (u: uPlot, ticks: number[]): string[] => {
  return ticks.map(v => {
    const n = Math.abs(v);
    if (n > 1e-3 && n < 1e4) {
      return v.toString();
    }
    return v.toExponential(1);
  });
};

export const getColorLine = (scale: number, label: string): string => getColorFromString(`${scale}${label}`);

export const getDashLine = (group: number): number[] => group <= 1 ? [] : [group*4, group*1.2];
