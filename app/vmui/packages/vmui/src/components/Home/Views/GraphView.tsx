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
    const allTimes = times.sort((a,b) => a-b);
    const startTime = allTimes[0] || 0;
    const endTime = allTimes[allTimes.length - 1] || 0;
    const step = period.step || 1;
    const length = Math.round((endTime - startTime) / step);
    setTimeArray(new Array(length).fill(0).map((d, i) => startTime + (step * i)));
  };

  const setLimitsYaxis = (values: number[]) => {
    if (!yaxis.limits.enable || (yaxis.limits.range.every(item => !item))) {
      const allValues = values.flat().sort((a,b) => a-b);
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
    const tempLegend: LegendItem[] = [];
    const tempSeries: uPlotSeries[] = [];

    data?.forEach(d => {
      const seriesItem = getSeriesItem(d);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem));

      d.values.forEach(v => {
        if (tempTimes.indexOf(v[0]) === -1) tempTimes.push(v[0]);
        tempValues.push(+v[1]);
      });
    });

    setTimes(tempTimes);
    setLimitsYaxis(tempValues);
    setSeries([{}, ...tempSeries]);
    setLegend(tempLegend);
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