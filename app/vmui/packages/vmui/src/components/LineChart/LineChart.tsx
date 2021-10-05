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

const LineChart: FC<GraphViewProps> = ({data = []}) => {

  const dispatch = useAppDispatch();
  const {time: {period}} = useAppState();
  const [dataChart, setDataChart] = useState<uPlotData>();
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [scale, setScale] = useState({min: period.start, max: period.end});
  const refContainer = useRef<HTMLDivElement>(null);

  const getColorByName = (str: string): string => {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }
    let colour = "#";
    for (let i = 0; i < 3; i++) {
      const value = (hash >> (i * 8)) & 0xFF;
      colour += ("00" + value.toString(16)).substr(-2);
    }
    return colour;
  };

  const times = useMemo(() => {
    const allTimes = data.map(d => d.values.map(v => v[0])).flat(); //.filter(t => t >= scale.min && t <= scale.max);
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
    const seriesValues = data.map(d => ({
      label: getNameForMetric(d),
      width: 1,
      font: "11px Arial",
      stroke: getColorByName(getNameForMetric(d))}));
    setSeries([{}, ...seriesValues]);
    setDataChart([times, ...values]);
  }, [data]);

  const onReadyChart = (u: uPlot) => {
    const factor = 0.85;

    // wheel drag pan
    u.over.addEventListener("mousedown", e => {
      if (e.button !== 0) return;
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
        document.removeEventListener("mousemove", onmove);
        document.removeEventListener("mouseup", onup);
      };

      document.addEventListener("mousemove", onmove);
      document.addEventListener("mouseup", onup);
    });

    // wheel scroll zoom
    u.over.addEventListener("wheel", e => {
      if (!e.ctrlKey) return;
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
    // TODO add check zoom in
    const duration = (period.end - period.start)/2;
    if (scale.max > period.end + duration || scale.min < period.start - duration) {
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
    scales: {x: {range: () => [scale.min, scale.max]}}
  };

  return <div ref={refContainer}>
    {dataChart && <UplotReact
      options={options}
      data={dataChart}
    />}
  </div>;
};

export default LineChart;