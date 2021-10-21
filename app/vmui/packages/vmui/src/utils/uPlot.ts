import uPlot, {AlignedData, Series} from "uplot";
import {getColorFromString} from "./color";
import dayjs from "dayjs";
import {MetricResult} from "../api/types";
import {getNameForMetric} from "./metric";
import {LegendItem} from "../components/Legend/Legend";

interface SetupTooltip {
    u: uPlot,
    data: MetricResult[],
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

export const setTooltip = ({ u, tooltipIdx, data, series, tooltip, tooltipOffset }: SetupTooltip) : void => {
  const {seriesIdx, dataIdx} = tooltipIdx;
  const dataSeries = u.data[seriesIdx][dataIdx];
  const dataTime = u.data[0][dataIdx];
  const metric = data[seriesIdx - 1]?.metric || {};
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
                         ${marker}${metric.__name__ || ""}: <b>${dataSeries}</b>
                       </div>
                       <div class="u-tooltip__info">${info}</div>`;
};

export const getSeries = (data: MetricResult[], hideSeries: string[]): Series[] => [{}, ...data.map(d => {
  const label = getNameForMetric(d);
  return {
    label,
    width: 1.5,
    stroke: getColorFromString(label),
    show: !hideSeries.includes(label)
  };
})];

export const getLegend = (series: Series[]): LegendItem[] => series.slice(1).map(s => ({
  label: s.label || "",
  color: s.stroke as string,
  checked: s.show || false
}));

export const getLimitsTimes = (data: MetricResult[]): [number, number] => {
  const allTimes = data.map(d => d.values.map(v => v[0])).flat().sort((a,b) => a-b);
  return [allTimes[0], allTimes[allTimes.length - 1]];
};

export const getLimitsYaxis = (data: MetricResult[]): [number, number] => {
  const allValues = data.map(d => d.values.map(v => +v[1])).flat().sort((a,b) => a-b);
  return [allValues[0], allValues[allValues.length - 1]];
};

export const getDataChart = (data: MetricResult[], times: number[]): AlignedData => {
  return [times, ...data.map(d => times.map(t => {
    const v = d.values.find(v => v[0] === t);
    return v ? +v[1] : null;
  }))];
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