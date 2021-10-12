import React, {FC, useEffect, useMemo, useRef, useState} from "react";
import {getNameForMetric} from "../../utils/metric";
import "chartjs-adapter-date-fns";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {GraphViewProps} from "../Home/Views/GraphView";
import uPlot, {AlignedData as uPlotData, Options as uPlotOptions, Series as uPlotSeries} from "uplot";
import UplotReact from "uplot-react";
import "uplot/dist/uPlot.min.css";
import numeral from "numeral";
import "./legend.css";
import {getColorFromString} from "../../utils/color";
import {useGraphDispatch, useGraphState} from "../../state/graph/GraphStateContext";

const LineChart: FC<GraphViewProps> = ({data = []}) => {

  const dispatch = useAppDispatch();
  const {time: {period}} = useAppState();
  const [dataChart, setDataChart] = useState<uPlotData>();
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [scale, setScale] = useState({min: period.start, max: period.end});
  const refContainer = useRef<HTMLDivElement>(null);
  const [isPanning, setIsPanning] = useState(false);

  const {yaxis} = useGraphState();
  const graphDispatch = useGraphDispatch();
  const setStateLimits = (range: [number, number]) => {
    if (!yaxis.limits.enable || (yaxis.limits.range.every(item => !item))) {
      graphDispatch({type: "SET_YAXIS_LIMITS", payload: range});
    }
  };

  const times = useMemo(() => {
    const allTimes = data.map(d => d.values.map(v => v[0])).flat();
    const start = Math.min(...allTimes);
    const end = Math.max(...allTimes);
    const output = [];
    for (let i = start; i < end; i += period.step || 1) {
      output.push(i);
    }
    return output;
  }, [data]);

  useEffect(() => {
    const values = data.map(d => times.map(t => {
      const v = d.values.find(v => v[0] === t);
      return v ? +v[1] : null;
    }));
    const flattenValues: number[] = values.flat().filter((item): item is number => item !== null);
    setStateLimits([Math.min(...flattenValues), Math.max(...flattenValues)]);
    const seriesValues = data.map(d => ({
      label: getNameForMetric(d),
      width: 1,
      font: "11px Arial",
      stroke: getColorFromString(getNameForMetric(d))}));
    setSeries([{}, ...seriesValues]);
    setDataChart([times, ...values]);
  }, [data]);

  const onReadyChart = (u: uPlot) => {
    const factor = 0.85;

    // wheel drag pan
    u.over.addEventListener("mousedown", e => {
      if (e.button !== 0) return;
      setIsPanning(true);
      e.preventDefault();
      const left0 = e.clientX;
      const scXMin0 = u.scales.x.min || 1;
      const scXMax0 = u.scales.x.max || 1;
      const xUnitsPerPx = u.posToVal(1, "x") - u.posToVal(0, "x");

      const onmove = (e: MouseEvent) => {
        e.preventDefault();
        const dx = xUnitsPerPx * (e.clientX - left0);
        const min = scXMin0 - dx;
        const max = scXMax0 - dx;
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
      const {left = width/2} = u.cursor;
      const leftPct = left/width;
      const xVal = u.posToVal(left, "x");
      const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
      const nxRange = e.deltaY < 0 ? oxRange * factor : oxRange / factor;
      const min = xVal - leftPct * nxRange;
      const max = min + nxRange;
      u.batch(() => {
        u.setScale("x", {min, max});
        setScale({min, max});
      });
    });
  };

  useEffect(() => {setScale({min: period.start, max: period.end});}, [period]);

  useEffect(() => {
    const duration = (period.end - period.start)/3;
    const factor = duration / (scale.max - scale.min);
    if (scale.max > period.end + duration || scale.min < period.start - duration || factor >= 0.7) {
      dispatch({type: "SET_PERIOD", payload: {from: new Date(scale.min * 1000), to: new Date(scale.max * 1000)}});
    }
  }, [scale]);

  const options: uPlotOptions = {
    width: refContainer.current ? refContainer.current.offsetWidth : 400,
    height: 500,
    series: series,
    plugins: [{
      hooks: {
        ready: onReadyChart
      }
    }],
    cursor: {drag: {x: false, y: false}},
    axes: [
      {space: 80},
      {
        show: true,
        font: "10px Arial",
        values: (self, ticks) => ticks.map(n => n > 1000 ? numeral(n).format("0.0a") : n)
      }
    ],
    scales: {
      x: {range: () => [scale.min, scale.max]},
      y: {range: (self, min, max) => yaxis.limits.enable ? yaxis.limits.range : [min, max]}
    }
  };

  return <div ref={refContainer} style={{pointerEvents: isPanning ? "none" : "auto"}}>
    {dataChart && <UplotReact options={options} data={dataChart}/>}
  </div>;
};

export default LineChart;