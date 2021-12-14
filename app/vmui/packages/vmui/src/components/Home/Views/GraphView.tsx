import React, {FC, useEffect, useState} from "react";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";
import {AlignedData as uPlotData, Series as uPlotSeries} from "uplot";
import Legend from "../../Legend/Legend";
import {useGraphDispatch} from "../../../state/graph/GraphStateContext";
import {getHideSeries, getLegendItem, getSeriesItem} from "../../../utils/uplot/series";
import {getLimitsYAxis, getTimeSeries} from "../../../utils/uplot/axes";
import {LegendItem} from "../../../utils/uplot/types";
import {AxisRange} from "../../../state/graph/reducer";
import GraphSettings from "../Configurator/Graph/GraphSettings";

export interface GraphViewProps {
  data?: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data = []}) => {
  const graphDispatch = useGraphDispatch();

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItem[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);
  const [valuesLimit, setValuesLimit] = useState<AxisRange>({"1": [0, 1]});

  const setLimitsYaxis = (values: {[key: string]: number[]}) => {
    const limits = getLimitsYAxis(values);
    setValuesLimit(limits);
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: limits});
  };

  const onChangeLegend = (legend: LegendItem, metaKey: boolean) => {
    setHideSeries(getHideSeries({hideSeries, legend, metaKey, series}));
  };

  useEffect(() => {
    const tempTimes: number[] = [];
    const tempValues: {[key: string]: number[]} = {};
    const tempLegend: LegendItem[] = [];
    const tempSeries: uPlotSeries[] = [];

    data?.forEach((d) => {
      const seriesItem = getSeriesItem(d, hideSeries);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem, d.group));

      d.values.forEach(v => {
        tempTimes.push(v[0]);
        tempValues[d.group] ? tempValues[d.group].push(+v[1]) : tempValues[d.group] = [+v[1]];
      });
    });

    const timeSeries = getTimeSeries(tempTimes);
    setDataChart([timeSeries, ...data.map(d => {
      return new Array(timeSeries.length).fill(1).map((v, i) => d.values[i] ? +d.values[i][1] : null);
    })] as uPlotData);
    setLimitsYaxis(tempValues);

    const newSeries = [{}, ...tempSeries];
    if (JSON.stringify(newSeries) !== JSON.stringify(series)) {
      setSeries(newSeries);
      setLegend(tempLegend);
    }
  }, [data]);

  useEffect(() => {
    const tempLegend: LegendItem[] = [];
    const tempSeries: uPlotSeries[] = [];
    data?.forEach(d => {
      const seriesItem = getSeriesItem(d, hideSeries);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem, d.group));
    });
    setSeries([{}, ...tempSeries]);
    setLegend(tempLegend);
  }, [hideSeries]);

  return <>
    {(data.length > 0)
      ? <div>
        <GraphSettings/>
        <LineChart data={dataChart} series={series} metrics={data} limits={valuesLimit}/>
        <Legend labels={legend} onChange={onChangeLegend}/>
      </div>
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;