import uPlot, {Series, Axis} from "uplot";
import {getColorFromString} from "./color";
import dayjs from "dayjs";
import {MetricResult} from "../api/types";
import {LegendItem} from "../components/Legend/Legend";
import {getNameForMetric} from "./metric";
import {getMaxFromArray, getMinFromArray} from "./math";
import {roundTimeSeconds} from "./time";
import numeral from "numeral";

interface SetupTooltip {
    u: uPlot,
    metrics: MetricResult[],
    series: Series[],
    tooltip: HTMLDivElement,
    tooltipOffset: {left: number, top: number},
    tooltipIdx: {seriesIdx: number, dataIdx: number}
}
interface HideSeriesArgs {
  hideSeries: string[],
  label: string,
  metaKey: boolean,
  series: Series[]
}
interface DragArgs {
  e: MouseEvent,
  u: uPlot,
  factor: number,
  setPanning: (enable: boolean) => void,
  setPlotScale: ({u, min, max}: {u: uPlot, min: number, max: number}) => void
}

const stub = (): null => null;
export const defaultOptions = {
  height: 500,
  legend: { show: false },
  cursor: {
    drag: { x: false, y: false },
    focus: { prox: 30 },
    bind: { mouseup: stub, mousedown: stub, click: stub, dblclick: stub, mouseenter: stub }
  },
};

export const formatTicks = (u: uPlot, ticks: number[]): (string | number)[] => {
  return ticks.map(n => n > 1000 ? numeral(n).format("0.0a") : n);
};
export const getAxes = (series: Series[]): Axis[] => Array.from(new Set(series.map(s => s.scale))).map(a => {
  const axis = { scale: a, show: true, font: "10px Arial", values: formatTicks };
  if (!(Number(a)%2)) return {...axis, side: 1};
  return a ? axis : { space: 80 };
});

export const setTooltip = ({ u, tooltipIdx, metrics, series, tooltip, tooltipOffset }: SetupTooltip) : void => {
  const {seriesIdx, dataIdx} = tooltipIdx;
  const dataSeries = u.data[seriesIdx][dataIdx];
  const dataTime = u.data[0][dataIdx];
  const metric = metrics[seriesIdx - 1]?.metric || {};
  const color = getColorFromString(series[seriesIdx].label || "");

  const {width, height} = u.over.getBoundingClientRect();
  const top = u.valToPos((dataSeries || 0), "y");
  const lft = u.valToPos(dataTime, "x");
  const {width: tooltipWidth, height: tooltipHeight} = tooltip.getBoundingClientRect();
  const overflowX = lft + tooltipWidth >= width;
  const overflowY = top + tooltipHeight >= height;

  tooltip.style.display = "grid";
  tooltip.style.top = `${tooltipOffset.top + top + 10 - (overflowY ? tooltipHeight + 10 : 0)}px`;
  tooltip.style.left = `${tooltipOffset.left + lft + 10 - (overflowX ? tooltipWidth + 20 : 0)}px`;
  const date = dayjs(new Date(dataTime * 1000)).format("YYYY-MM-DD HH:mm:ss:SSS (Z)");
  const info = Object.keys(metric).filter(k => k !== "__name__").map(k => `<div><b>${k}</b>: ${metric[k]}</div>`).join("");
  const marker = `<div class="u-tooltip__marker" style="background: ${color}"></div>`;
  tooltip.innerHTML = `<div>${date}</div>
                       <div class="u-tooltip-data">
                         ${marker}${metric.__name__ || ""}: <b class="u-tooltip-data__value">${dataSeries}</b>
                       </div>
                       <div class="u-tooltip__info">${info}</div>`;
};

export const getHideSeries = ({hideSeries, label, metaKey, series}: HideSeriesArgs): string[] => {
  const include = hideSeries.includes(label);
  const labels = series.map(s => s.label || "").filter(l => l);
  if (metaKey && include) {
    return [...labels.filter(l => l !== label)];
  } else if (metaKey && !include) {
    return hideSeries.length === series.length - 2 ? [] : [...labels.filter(l => l !== label)];
  }
  return include ? hideSeries.filter(l => l !== label) : [...hideSeries, label];
};

export const getTimeSeries = (times: number[]): number[] => {
  const allTimes = Array.from(new Set(times)).sort((a,b) => a-b);
  const step = getMinFromArray(allTimes.map((t, i) => allTimes[i + 1] - t));
  const length = allTimes.length;
  const startTime = allTimes[0] || 0;
  return new Array(length).fill(startTime).map((d, i) => roundTimeSeconds(d + (step * i)));
};

export const getLimitsYAxis = (values: number[]): [number, number] => {
  const min = getMinFromArray(values);
  const max = getMaxFromArray(values);
  return [min - (min * 0.05), max + (max * 0.05)];
};

export const getSeriesItem = (d: MetricResult, hideSeries: string[]): Series  => {
  const label = getNameForMetric(d);
  return {
    label,
    dash: d.group <= 1 ? [] : [10, d.group * 2],
    width: 1.5,
    stroke: getColorFromString(label),
    show: !hideSeries.includes(label),
    scale: String(d.group)
  };
};

export const getLegendItem = (s: Series, group: number): LegendItem => ({
  group, label: s.label || "", color: s.stroke as string, checked: s.show || false
});

export const dragChart = ({e, factor = 0.85, u, setPanning, setPlotScale}: DragArgs): void => {
  if (e.button !== 0) return;
  e.preventDefault();
  setPanning(true);
  const leftStart = e.clientX;
  const xUnitsPerPx = u.posToVal(1, "x") - u.posToVal(0, "x");
  const scXMin = u.scales.x.min || 0;
  const scXMax = u.scales.x.max || 0;

  const mouseMove = (e: MouseEvent) => {
    e.preventDefault();
    const dx = xUnitsPerPx * ((e.clientX - leftStart) * factor);
    setPlotScale({u, min: scXMin - dx, max: scXMax - dx});
  };
  const mouseUp = () => {
    setPanning(false);
    document.removeEventListener("mousemove", mouseMove);
    document.removeEventListener("mouseup", mouseUp);
  };

  document.addEventListener("mousemove", mouseMove);
  document.addEventListener("mouseup", mouseUp);
};