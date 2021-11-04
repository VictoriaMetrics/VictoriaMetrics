import React, {FC, useRef, useState} from "react";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import uPlot, {AlignedData as uPlotData, Options as uPlotOptions, Series as uPlotSeries} from "uplot";
import UplotReact from "uplot-react";
import "uplot/dist/uPlot.min.css";
import numeral from "numeral";
import "./tooltip.css";
import {useGraphState} from "../../state/graph/GraphStateContext";
import {setTooltip } from "../../utils/uPlot";
import {MetricResult} from "../../api/types";

export interface LineChartProps {
  metrics: MetricResult[]
  data: uPlotData;
  series: uPlotSeries[]
}

const LineChart: FC<LineChartProps> = ({data, series, metrics = []}) => {
  const dispatch = useAppDispatch();
  const {time: {period}} = useAppState();
  const { yaxis } = useGraphState();
  const refContainer = useRef<HTMLDivElement>(null);
  const [isPanning, setIsPanning] = useState(false);
  const [zoomPos, setZoomPos] = useState(0);
  const tooltipIdx = { seriesIdx: 1, dataIdx: 0 };
  const tooltipOffset = { left: 0, top: 0 };

  const tooltip = document.createElement("div");
  tooltip.className = "u-tooltip";

  const onReadyChart = (u: uPlot) => {
    const factor = 0.85;
    tooltipOffset.left = parseFloat(u.over.style.left);
    tooltipOffset.top = parseFloat(u.over.style.top);
    u.root.querySelector(".u-wrap")?.appendChild(tooltip);
    // wheel drag pan
    u.over.addEventListener("mousedown", e => {
      if (e.button !== 0) return;
      setIsPanning(true);
      e.preventDefault();
      const left0 = e.clientX;
      const onmove = (e: MouseEvent) => {
        e.preventDefault();
        const dx = (u.posToVal(1, "x") - u.posToVal(0, "x")) * (e.clientX - left0);
        const min = (u.scales.x.min || 1) - dx;
        const max = (u.scales.x.max || 1) - dx;
        u.setScale("x", {min, max});
        setScale({min, max});
      };
      const onup = () => {
        setIsPanning(false);
        document.removeEventListener("mousemove", onmove);
        document.removeEventListener("mouseup", onup);
      };
      document.addEventListener("mousemove", onmove);
      document.addEventListener("mouseup", onup);
    });
    // wheel scroll zoom
    u.over.addEventListener("wheel", e => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      const {width} = u.over.getBoundingClientRect();
      if (u.cursor.left && u.cursor.left > 0) setZoomPos(u.cursor.left);
      const xVal = u.posToVal(zoomPos, "x");
      const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
      const nxRange = e.deltaY < 0 ? oxRange * factor : oxRange / factor;
      const min = xVal - (zoomPos/width) * nxRange;
      const max = min + nxRange;
      u.batch(() => {
        u.setScale("x", {min, max});
        setScale({min, max});
      });
    });
  };

  const setCursor = (u: uPlot) => {
    if (tooltipIdx.dataIdx === u.cursor.idx) return;
    tooltipIdx.dataIdx = u.cursor.idx || 0;
    if (tooltipIdx.seriesIdx && tooltipIdx.dataIdx) {
      setTooltip({u, tooltipIdx, metrics, series, tooltip, tooltipOffset});
    }
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    if (tooltipIdx.seriesIdx === sidx) return;
    tooltipIdx.seriesIdx = sidx || 0;
    sidx && tooltipIdx.dataIdx
      ? setTooltip({u, tooltipIdx, metrics, series, tooltip, tooltipOffset})
      : tooltip.style.display = "none";
  };

  const setScale = ({min, max}: {min: number, max: number}): void => {
    dispatch({type: "SET_PERIOD", payload: {from: new Date(min * 1000), to: new Date(max * 1000)}});
  };

  const options: uPlotOptions = {
    width: refContainer.current ? refContainer.current.offsetWidth : 400, height: 500, series: series,
    plugins: [{ hooks: { ready: onReadyChart, setCursor, setSeries: seriesFocus }}],
    cursor: {
      drag: { x: false, y: false },
      focus: { prox: 30 },
      bind: {
        mouseup: () => null,
        mousedown: () => null,
        click: () => null,
        dblclick: () => null,
        mouseenter: () => null,
      }
    },
    legend: { show: false },
    axes: [
      { space: 80 },
      { show: true, font: "10px Arial",
        values: (self, ticks) => ticks.map(n => n > 1000 ? numeral(n).format("0.0a") : n) }
    ],
    scales: {
      x: { range: () => [period.start, period.end] },
      y: {
        range: (self, min, max) => {
          const offsetFactor = 0.05; // 5%
          return yaxis.limits.enable ? yaxis.limits.range : [min - (min * offsetFactor), max + (max * offsetFactor)];
        }
      }
    }
  };

  return <div ref={refContainer} style={{pointerEvents: isPanning ? "none" : "auto", height: "500px"}}>
    <UplotReact options={options} data={data}/>
  </div>;
};

export default LineChart;