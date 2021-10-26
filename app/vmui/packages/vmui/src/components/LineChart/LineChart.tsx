import React, {FC, useEffect, useMemo, useRef, useState} from "react";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {GraphViewProps} from "../Home/Views/GraphView";
import uPlot, {AlignedData as uPlotData, Options as uPlotOptions, Series as uPlotSeries} from "uplot";
import UplotReact from "uplot-react";
import "uplot/dist/uPlot.min.css";
import numeral from "numeral";
import "./tooltip.css";
import {useGraphDispatch, useGraphState} from "../../state/graph/GraphStateContext";
import {getDataChart, getHideSeries, getLegend, getLimitsTimes, getLimitsYaxis, getSeries, setTooltip } from "../../utils/uPlot";
import {Legend, LegendItem} from "../Legend/Legend";

const LineChart: FC<GraphViewProps> = ({data = []}) => {
  const dispatch = useAppDispatch();
  const {time: {period}} = useAppState();
  const graphDispatch = useGraphDispatch();
  const { yaxis } = useGraphState();
  const refContainer = useRef<HTMLDivElement>(null);
  const [isPanning, setIsPanning] = useState(false);
  const [zoomPos, setZoomPos] = useState(0);
  const [hideSeries, setHideSeries] = useState<string[]>([]);
  const tooltipIdx = { seriesIdx: 1, dataIdx: 0 };
  const tooltipOffset = { left: 0, top: 0 };

  const setStateLimits = (range: [number, number]) => {
    if (!yaxis.limits.enable || (yaxis.limits.range.every(item => !item))) {
      graphDispatch({type: "SET_YAXIS_LIMITS", payload: range});
    }
  };

  const times = useMemo(() => {
    const [start, end] = getLimitsTimes(data);
    const output = [];
    for (let i = start; i < end; i += period.step || 1) { output.push(i); }
    return output;
  }, [data]);
  const series = useMemo((): uPlotSeries[] => getSeries(data, hideSeries), [data, hideSeries]);
  const dataChart = useMemo((): uPlotData => getDataChart(data, times), [data]);
  const legend = useMemo((): LegendItem[] => getLegend(series), [series]);

  const tooltip = document.createElement("div");
  tooltip.className = "u-tooltip";

  const onChangeLegend = (label: string, metaKey: boolean) => {
    setHideSeries(getHideSeries({hideSeries, label, metaKey, series}));
  };

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
      setTooltip({u, tooltipIdx, data, series, tooltip, tooltipOffset});
    }
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    if (tooltipIdx.seriesIdx === sidx) return;
    tooltipIdx.seriesIdx = sidx || 0;
    sidx && tooltipIdx.dataIdx
      ? setTooltip({u, tooltipIdx, data, series, tooltip, tooltipOffset})
      : tooltip.style.display = "none";
  };

  const setScale = ({min, max}: {min: number, max: number}): void => {
    dispatch({type: "SET_PERIOD", payload: {from: new Date(min * 1000), to: new Date(max * 1000)}});
  };

  useEffect(() => { setStateLimits(getLimitsYaxis(data)); }, [data]);

  const options: uPlotOptions = {
    width: refContainer.current ? refContainer.current.offsetWidth : 400, height: 500, series: series,
    plugins: [{ hooks: { ready: onReadyChart, setCursor, setSeries: seriesFocus }}],
    cursor: { drag: { x: false, y: false }, focus: { prox: 30 }},
    legend: { show: false },
    axes: [
      { space: 80 },
      { show: true, font: "10px Arial",
        values: (self, ticks) => ticks.map(n => n > 1000 ? numeral(n).format("0.0a") : n) }
    ],
    scales: {
      x: {range: () => [period.start, period.end]},
      y: {range: (self, min, max) => yaxis.limits.enable ? yaxis.limits.range : [min, max]}
    }
  };

  return <div ref={refContainer} style={{pointerEvents: isPanning ? "none" : "auto"}}>
    {dataChart && <>
      <UplotReact options={options} data={dataChart}/>
      <Legend labels={legend} onChange={onChangeLegend}/>
    </>}
  </div>;
};

export default LineChart;