import React, {FC, useCallback, useEffect, useRef, useState} from "preact/compat";
import uPlot, {AlignedData as uPlotData, Options as uPlotOptions, Series as uPlotSeries, Range, Scales, Scale} from "uplot";
import {defaultOptions} from "../../utils/uplot/helpers";
import {dragChart} from "../../utils/uplot/events";
import {getAxes, getMinMaxBuffer} from "../../utils/uplot/axes";
import {setTooltip} from "../../utils/uplot/tooltip";
import {MetricResult} from "../../api/types";
import {limitsDurations} from "../../utils/time";
import throttle from "lodash.throttle";
import "uplot/dist/uPlot.min.css";
import "./tooltip.css";
import useResize from "../../hooks/useResize";
import {TimeParams} from "../../types";
import {YaxisState} from "../../state/graph/reducer";

export interface LineChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  yaxis: YaxisState;
  series: uPlotSeries[];
  unit?: string;
  setPeriod: ({from, to}: {from: Date, to: Date}) => void;
  container: HTMLDivElement | null
}
enum typeChartUpdate {xRange = "xRange", yRange = "yRange", data = "data"}

const LineChart: FC<LineChartProps> = ({data, series, metrics = [],
  period, yaxis, unit, setPeriod, container}) => {

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [isPanning, setPanning] = useState(false);
  const [xRange, setXRange] = useState({min: period.start, max: period.end});
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const layoutSize = useResize(container);

  const tooltip = document.createElement("div");
  tooltip.className = "u-tooltip";
  const tooltipIdx: {seriesIdx: number | null, dataIdx: number | undefined} = {seriesIdx: null, dataIdx: undefined};
  const tooltipOffset = {left: 0, top: 0};

  const setScale = ({min, max}: { min: number, max: number }): void => {
    setPeriod({from: new Date(min * 1000), to: new Date(max * 1000)});
  };
  const throttledSetScale = useCallback(throttle(setScale, 500), []);
  const setPlotScale = ({u, min, max}: { u: uPlot, min: number, max: number }) => {
    const delta = (max - min) * 1000;
    if ((delta < limitsDurations.min) || (delta > limitsDurations.max)) return;
    u.setScale("x", {min, max});
    setXRange({min, max});
    throttledSetScale({min, max});
  };

  const onReadyChart = (u: uPlot) => {
    const factor = 0.9;
    tooltipOffset.left = parseFloat(u.over.style.left);
    tooltipOffset.top = parseFloat(u.over.style.top);
    u.root.querySelector(".u-wrap")?.appendChild(tooltip);
    // wheel drag pan
    u.over.addEventListener("mousedown", e => dragChart({u, e, setPanning, setPlotScale, factor}));
    // wheel scroll zoom
    u.over.addEventListener("wheel", e => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      const {width} = u.over.getBoundingClientRect();
      const zoomPos = u.cursor.left && u.cursor.left > 0 ? u.cursor.left : 0;
      const xVal = u.posToVal(zoomPos, "x");
      const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
      const nxRange = e.deltaY < 0 ? oxRange * factor : oxRange / factor;
      const min = xVal - (zoomPos / width) * nxRange;
      const max = min + nxRange;
      u.batch(() => setPlotScale({u, min, max}));
    });
  };

  const setCursor = (u: uPlot) => {
    if (tooltipIdx.dataIdx === u.cursor.idx) return;
    tooltipIdx.dataIdx = u.cursor.idx || 0;
    if (tooltipIdx.seriesIdx !== null && tooltipIdx.dataIdx !== undefined) {
      setTooltip({u, tooltipIdx, metrics, series, tooltip, tooltipOffset, unit});
    }
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    if (tooltipIdx.seriesIdx === sidx) return;
    tooltipIdx.seriesIdx = sidx;
    sidx && tooltipIdx.dataIdx !== undefined
      ? setTooltip({u, tooltipIdx, metrics, series, tooltip, tooltipOffset, unit})
      : tooltip.style.display = "none";
  };
  const getRangeX = (): Range.MinMax => [xRange.min, xRange.max];
  const getRangeY = (u: uPlot, min = 0, max = 1, axis: string): Range.MinMax => {
    if (yaxis.limits.enable) return yaxis.limits.range[axis];
    return getMinMaxBuffer(min, max);
  };

  const getScales = (): Scales => {
    const scales: { [key: string]: { range: Scale.Range } } = {x: {range: getRangeX}};
    Object.keys(yaxis.limits.range).forEach(axis => {
      scales[axis] = {range: (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis)};
    });
    return scales;
  };

  const options: uPlotOptions = {
    ...defaultOptions,
    series,
    axes: getAxes(series, unit),
    scales: {...getScales()},
    width: layoutSize.width || 400,
    plugins: [{hooks: {ready: onReadyChart, setCursor, setSeries: seriesFocus}}],
  };

  const updateChart = (type: typeChartUpdate): void => {
    if (!uPlotInst) return;
    switch (type) {
      case typeChartUpdate.xRange:
        uPlotInst.scales.x.range = getRangeX;
        break;
      case typeChartUpdate.yRange:
        Object.keys(yaxis.limits.range).forEach(axis => {
          if (!uPlotInst.scales[axis]) return;
          uPlotInst.scales[axis].range = (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis);
        });
        break;
      case typeChartUpdate.data:
        uPlotInst.setData(data);
        break;
    }
    if (!isPanning) uPlotInst.redraw();
  };

  useEffect(() => setXRange({min: period.start, max: period.end}), [period]);

  useEffect(() => {
    if (!uPlotRef.current) return;
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    setXRange({min: period.start, max: period.end});
    return u.destroy;
  }, [uPlotRef.current, series, layoutSize]);

  useEffect(() => updateChart(typeChartUpdate.data), [data]);
  useEffect(() => updateChart(typeChartUpdate.xRange), [xRange]);
  useEffect(() => updateChart(typeChartUpdate.yRange), [yaxis]);

  return <div style={{pointerEvents: isPanning ? "none" : "auto", height: "500px"}}>
    <div ref={uPlotRef}/>
  </div>;
};

export default LineChart;
