import React, {FC, useMemo} from "react";
import {Line} from "react-chartjs-2";
import {Chart, ChartData, ChartOptions, ScatterDataPoint, TimeScale} from "chart.js";
import zoomPlugin from "chartjs-plugin-zoom";
import {MetricResult} from "../../api/types";
import {getNameForMetric} from "../../utils/metric";
import "chartjs-adapter-date-fns";
import debounce from "lodash.debounce";
import {TimePeriod} from "../../types";
import {useAppDispatch} from "../../state/common/StateContext";
Chart.register(zoomPlugin);

export interface LineChartProps {
    data: MetricResult[];
}

const LineChart: FC<LineChartProps> = ({data}) => {
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

  const onZoomComplete = ({chart}: {chart: Chart}) => {
    // TODO add limits
    const {min = 0, max = 0} = (chart.boxes.find(box => box.constructor.name === "TimeScale") || {}) as TimeScale;
    if (!min || !max) return;
    const period: TimePeriod = {
      from: new Date(min),
      to: new Date(max)
    };
    dispatch({type: "SET_PERIOD", payload: period});
  };

  const onPanComplete = ({chart}: {chart: Chart}) => {
    // TODO check min/max. Prevent duration changes
    const {min = 0, max = 0} = (chart.boxes.find(box => box.constructor.name === "TimeScale") || {}) as TimeScale;
    if (!min || !max) return;
    const period: TimePeriod = {
      from: new Date(min),
      to: new Date(max)
    };
    dispatch({type: "SET_PERIOD", payload: period});
  };

  const options: ChartOptions = {
    animation: false,
    scales: {
      x: {
        type: "time",
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
          onPanComplete:  debounce(onPanComplete, 500)
        },
        zoom: {
          wheel: {
            enabled: true
          },
          onZoomComplete: debounce(onZoomComplete, 500)
        }
      }
    }
  };


  return <>
    <Line data={series} options={options} />
  </>;
};

export default LineChart;