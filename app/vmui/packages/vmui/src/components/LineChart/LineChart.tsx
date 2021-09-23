import React, {FC, useEffect, useRef, useState} from "react";
import {Line} from "react-chartjs-2";
import {Chart, ChartData, ChartOptions, ScatterDataPoint} from "chart.js";
import {getNameForMetric} from "../../utils/metric";
import "chartjs-adapter-date-fns";
import debounce from "lodash.debounce";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {dateFromSeconds, getTimeperiodForDuration} from "../../utils/time";
import {GraphViewProps} from "../Home/Views/GraphView";
import {limitsDurations} from "../../utils/time";

const LineChart: FC<GraphViewProps> = ({data = []}) => {

  const {time: {duration, period}} = useAppState();
  const dispatch = useAppDispatch();
  const [series, setSeries] = useState<ChartData<"line", (ScatterDataPoint)[]>>();
  const refLine = useRef<Chart>(null);

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

  useEffect(() => {
    setSeries({
      datasets: data?.map(d => {
        const label = getNameForMetric(d);
        const color = getColorByName(label);
        return {
          label,
          data: d.values.map(v => ({y: +v[1], x: v[0] * 1000})),
          borderColor: color,
          backgroundColor: color,
        };
      })
    });
    if (refLine.current) {
      refLine.current.stop(); // make sure animations are not running
      refLine.current.update("none");
    }
  }, [data]);

  const onZoomComplete = ({chart}: {chart: Chart}) => {
    let {min, max} = chart.scales.x;
    if (!min || !max) return;
    const duration = max - min;
    if (duration < limitsDurations.min) max = min + limitsDurations.min;
    if (duration > limitsDurations.max) min = max - limitsDurations.max;
    dispatch({type: "SET_PERIOD", payload: {from: new Date(min), to: new Date(max)}});
  };

  const onPanComplete = ({chart}: {chart: Chart}) => {
    const {min, max} = chart.scales.x;
    if (!min || !max) return;
    const {start,  end} = getTimeperiodForDuration(duration, new Date(max));
    dispatch({type: "SET_PERIOD", payload: {from: dateFromSeconds(start), to: dateFromSeconds(end)}});
  };

  const options: ChartOptions = {
    animation: {duration: 0},
    parsing: false,
    normalized: true,
    scales: {
      x: {
        type: "time",
        position: "bottom",
        min: (period.start * 1000),
        max: (period.end * 1000),
        time: {
          tooltipFormat: "yyyy-MM-dd HH:mm:ss.SSS",
          displayFormats: {millisecond: ":ss.SSS", second: "HH:mm:ss", minute: "HH:mm", hour: "HH:mm"}
        },
        ticks: {
          source: "auto",
          autoSkip: true,
          autoSkipPadding: 105,
          crossAlign: "center",
          maxRotation: 0,
          minRotation: 0,
          sampleSize: 1,
          color: "#000",
          font: {size: 10}
        },
      },
      y: {
        type: "linear",
        position: "left",
        ticks: {
          maxRotation: 0,
          minRotation: 0,
          color: "#000",
          font: {size: 10}
        }
      }
    },
    elements: {
      line: {
        tension: 0,
        stepped: false,
        borderDash: [],
        borderWidth: 1,
        capBezierPoints: false
      },
      point: {radius: 0, hitRadius: 20}
    },
    plugins: {
      legend: {
        position: "bottom",
        align: "start",
        labels: {padding: 20, color: "#000"}
      },
      zoom: {
        pan: {
          enabled: true,
          mode: "x",
          onPanComplete: debounce(onPanComplete, 750)
        },
        zoom: {
          pinch: {enabled: true},
          wheel: {enabled: true, speed: 0.05},
          mode: "x",
          onZoomComplete: debounce(onZoomComplete, 250)
        }
      },
    }
  };

  return <>
    {series && <Line data={series} options={options} ref={refLine}/>}
  </>;
};

export default LineChart;