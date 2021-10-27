import uPlot, {Series} from "uplot";
import {getColorFromString} from "./color";
import dayjs from "dayjs";
import {MetricResult} from "../api/types";

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
                         ${marker}${metric.__name__ || ""}: <b>${dataSeries}</b>
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