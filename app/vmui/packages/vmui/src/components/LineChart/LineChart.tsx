import React, {FC, useEffect, useMemo, useRef, useState} from "react";
import {getNameForMetric} from "../../utils/metric";
import "chartjs-adapter-date-fns";
import {useAppState} from "../../state/common/StateContext";
import {GraphViewProps} from "../Home/Views/GraphView";
import uPlot, {AlignedData as uPlotData, Options as uPlotOptions, Series as uPlotSeries} from "uplot";
import UplotReact from "uplot-react";
import "uplot/dist/uPlot.min.css";
import numeral from "numeral";
import "./legend.css";

const LineChart: FC<GraphViewProps> = ({data = []}) => {

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
    const allTimes = data.map(d => d.values.map(v => v[0])).flat().filter(t => t >= scale.min && t <= scale.max);
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

  const wheelZoomPlugin = () => {
    const factor = 0.75;

    return {
      hooks: {
        ready: (u: uPlot) => {
          const over = u.over;
          const rect = over.getBoundingClientRect();

          // wheel scroll zoom
          over.addEventListener("wheel", e => {
            e.preventDefault();
            const {left = 0} = u.cursor;
            const leftPct = left/rect.width;
            const xVal = u.posToVal(left, "x");
            const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
            const nxRange = e.deltaY < 0 ? oxRange * factor : oxRange / factor;
            const min = xVal - leftPct * nxRange;
            const max = min + nxRange;
            // dispatch({type: "SET_PERIOD", payload: {from: new Date(min * 1000), to: new Date(max * 1000)}});
            // setScale({min, max});
            u.batch(() => {
              u.setScale("x", {min, max});
            });
          });
        }
      }
    };
  };

  const options: uPlotOptions = {
    width: refContainer.current ? refContainer.current.offsetWidth : 400,
    height: 500,
    series: series,
    plugins: [wheelZoomPlugin()],
    cursor: { drag: { x: false, y: false }},
    axes: [
      { space: 80},
      {
        show: true,
        font: "10px Arial",
        values: (self, ticks) => ticks.map(n => n > 1000 ? numeral(n).format("0.0a") : n)
      }
    ],
  };

  return <div ref={refContainer}>
    {dataChart && <UplotReact
      options={options}
      data={dataChart}
    />}
  </div>;
};

export default LineChart;