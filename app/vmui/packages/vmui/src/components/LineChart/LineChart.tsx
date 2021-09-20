import React, {FC, useMemo} from "react";
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

  const {time: {duration}} = useAppState();
  const dispatch = useAppDispatch();

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

  const series: ChartData<"line", (ScatterDataPoint)[], unknown> = useMemo(() => ({
    datasets: data?.map(d => {
      const label = getNameForMetric(d);
      const color = getColorByName(label);
      return {
        label,
        data: d.values.map(v => ({y: +v[1], x: v[0] * 1000})),
        borderColor: color,
        backgroundColor: color
      };
    })
  }), [data]);

  const getRangeTimeScale = (chart: Chart) => {
    const {min = 0, max = 0} = (chart.boxes.find(box => box.constructor.name === "TimeScale") || {}) as TimeScale;
    return {min, max};
  };

  const onZoomComplete = ({chart}: {chart: Chart}) => {
    const {min, max} = getRangeTimeScale(chart);
    if (!min || !max || (max - min < 1000)) return;
    const period: TimePeriod = {
      from: new Date(min),
      to: new Date(max)
    };
    dispatch({type: "SET_PERIOD", payload: period});
  };

  const onPanComplete = ({chart}: {chart: Chart}) => {
    const {min, max} = getRangeTimeScale(chart);
    if (!min || !max) return;
    const {start,  end} = getTimeperiodForDuration(duration, new Date(max));
    const period: TimePeriod = {
      from: dateFromSeconds(start),
      to: dateFromSeconds(end)
    };
    dispatch({type: "SET_PERIOD", payload: period});
  };

  const options: ChartOptions = {
    animation: {
      delay: 0,
      duration: 250,
      easing: "linear",
    },
    scales: {
      x: {
        type: "time",
        time: {
          tooltipFormat: "yyyy-MM-dd HH:mm:ss.SSS",
          displayFormats: {
            millisecond: "HH:mm:ss.SSS",
            second: "HH:mm:ss",
            minute: "HH:mm",
            hour: "HH:mm"
          },
        },
        ticks: {
          source: "auto",
          autoSkip: true,
        }
      }
    },
    plugins: {
      legend: {
        position: "bottom",
        align: "start",
        labels: {
          padding: 20,
          color: "#000"
        }
      },
      zoom: {
        pan: {
          enabled: true,
          mode: "x",
          onPanComplete:  debounce(onPanComplete, 500)
        },
        zoom: {
          wheel: {
            enabled: true,
          },
          mode: "x",
          onZoomComplete: debounce(onZoomComplete, 500)
        }
      }
    }
  };


  return <>
    <Line data={series} options={options} height={100} />
  </>;
};

export default LineChart;