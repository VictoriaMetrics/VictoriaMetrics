import uPlot, { Options as uPlotOptions } from "uplot";
import { delSeries } from "./series";
import { delHooks } from "./hooks";
import dayjs from "dayjs";
import { dateFromSeconds, formatDateForNativeInput } from "../time";

interface InstanceOptions {
  width?: number,
  height?: number
}

export const getDefaultOptions = ({ width = 400, height = 500 }: InstanceOptions): uPlotOptions => ({
  width,
  height,
  series: [],
  tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
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
});

export const handleDestroy = (u: uPlot) => {
  delSeries(u);
  delHooks(u);
  u.setData([]);
};
