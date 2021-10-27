import React, {FC, useEffect, useState} from "react";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";
import {AlignedData as uPlotData, Series as uPlotSeries} from "uplot";
import {Legend, LegendItem} from "../../Legend/Legend";
import {useAppState} from "../../../state/common/StateContext";
import {getNameForMetric} from "../../../utils/metric";
import {getColorFromString} from "../../../utils/color";
import {useGraphDispatch, useGraphState} from "../../../state/graph/GraphStateContext";
import {getHideSeries} from "../../../utils/uPlot";

export interface GraphViewProps {
  data?: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data = []}) => {
  const {time: {period}} = useAppState();

  const { yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const [timeArray, setTimeArray] = useState<number[]>([]);
  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItem[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);

  const setTimes = (times: number[]) => {
    const allTimes = times.flat().sort((a,b) => a-b);
    const output = [];
    for (let i = allTimes[0]; i < allTimes[allTimes.length - 1]; i += period.step || 1) {
      output.push(i);
    }
    setTimeArray(output);
  };

  const setLimitsYaxis = (values: number[]) => {
    const allValues = values.flat().sort((a,b) => a-b);
    if (!yaxis.limits.enable || (yaxis.limits.range.every(item => !item))) {
      graphDispatch({type: "SET_YAXIS_LIMITS", payload: [allValues[0], allValues[allValues.length - 1]]});
    }
  };

  const getSeriesItem = (d: MetricResult) => {
    const label = getNameForMetric(d);
    return {
      label,
      width: 1.5,
      stroke: getColorFromString(label),
      show: !hideSeries.includes(label)
    };
  };

  const getLegendItem = (s: uPlotSeries): LegendItem => ({
    label: s.label || "",
    color: s.stroke as string,
    checked: s.show || false
  });

  const onChangeLegend = (label: string, metaKey: boolean) => {
    setHideSeries(getHideSeries({hideSeries, label, metaKey, series}));
  };

  useEffect(() => {
    const tempTimes: number[] = [];
    const tempValues: number[] = [];

    data?.forEach(d => {
      d.values.forEach(v => {
        tempTimes.push(v[0]);
        tempValues.push(+v[1]);
      });
    });

    setTimes(tempTimes);
    setLimitsYaxis(tempValues);
  }, [data]);

  useEffect(() => {
    const tempLegend: LegendItem[] = [];
    const tempSeries: uPlotSeries[] = [];
    data?.forEach(d => {
      const seriesItem = getSeriesItem(d);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem));
    });
    setSeries([{}, ...tempSeries]);
    setLegend(tempLegend);
  }, [hideSeries]);

  useEffect(() => {
    setDataChart([timeArray, ...data.map(d => timeArray.map(t => {
      const v = d.values.find(v => v[0] === t);
      return v ? +v[1] : null;
    }))]);
  }, [timeArray]);

  return <>
    {(data.length > 0)
      ? <div>
        <LineChart data={dataChart} series={series} metrics={data}/>
        <Legend labels={legend} onChange={onChangeLegend}/>
      </div>
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;