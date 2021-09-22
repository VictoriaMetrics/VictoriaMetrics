import React, {FC, useEffect, useState} from "react";
import {Line} from "react-chartjs-2";
import {Chart, ChartData, ChartOptions, ScatterDataPoint, TimeScale} from "chart.js";
import {getNameForMetric} from "../../utils/metric";
import "chartjs-adapter-date-fns";
import debounce from "lodash.debounce";
import {TimePeriod} from "../../types";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import {dateFromSeconds, getTimeperiodForDuration} from "../../utils/time";
import {GraphViewProps} from "../Home/Views/GraphView";

const LineChart: FC<GraphViewProps> = ({data = []}) => {

  const {time: {duration, period}} = useAppState();
  const dispatch = useAppDispatch();
  const [series, setSeries] = useState<ChartData<"line", (ScatterDataPoint)[]>>();

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
  }, [data]);

  const getRangeTimeScale = (chart: Chart) => {
    const {min = 0, max = 0} = (chart.boxes.find(box => box.constructor.name === "TimeScale") || {}) as TimeScale;
    return {min, max};
  };

  const onZoomComplete = ({chart}: {chart: Chart}) => {
    const {min, max} = getRangeTimeScale(chart);
    if (!min || !max || (max - min < 1000)) return;
    const dateRange: TimePeriod = {from: new Date(min), to: new Date(max)};
    dispatch({type: "SET_PERIOD", payload: dateRange});
  };

  const onPanComplete = ({chart}: {chart: Chart}) => {
    const {min, max} = getRangeTimeScale(chart);
    if (!min || !max) return;
    const {start,  end} = getTimeperiodForDuration(duration, new Date(max));
    const dateRange: TimePeriod = {from: dateFromSeconds(start), to: dateFromSeconds(end)};
    dispatch({type: "SET_PERIOD", payload: dateRange});
  };

  const options: ChartOptions = {
    animation: {
      duration: 0,
      delay: 0,
    },
    animations: {type: false},
    parsing: false,
    normalized: true,
    scales: {
      x: {
        type: "time",
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
          maxRotation: 1,
          minRotation: 1,
          sampleSize: 1,
          color: "#000",
          font: {size: 10}
        },
      },
      y: {
        ticks: {
          maxRotation: 1,
          minRotation: 1,
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
      point: {radius: 0}
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
    {series &&  <Line data={series} options={options}/>}
  </>;
};

export default LineChart;