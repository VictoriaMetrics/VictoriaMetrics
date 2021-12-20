import dayjs from "dayjs";
import {SetupTooltip} from "./types";
import {getColorLine} from "./helpers";

export const setTooltip = ({u, tooltipIdx, metrics, series, tooltip, tooltipOffset}: SetupTooltip): void => {
  const {seriesIdx, dataIdx} = tooltipIdx;
  if (seriesIdx === null || dataIdx === undefined) return;
  const dataSeries = u.data[seriesIdx][dataIdx];
  const dataTime = u.data[0][dataIdx];
  const metric = metrics[seriesIdx - 1]?.metric || {};
  const color = getColorLine(Number(series[seriesIdx].scale || 0), series[seriesIdx].label || "");

  const {width, height} = u.over.getBoundingClientRect();
  const top = u.valToPos((dataSeries || 0), series[seriesIdx]?.scale || "1");
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
