import React, {FC, useEffect, useState} from "react";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";
import {AlignedData as uPlotData, Series as uPlotSeries} from "uplot";
import {Legend, LegendItem} from "../../Legend/Legend";
import {useGraphDispatch, useGraphState} from "../../../state/graph/GraphStateContext";
import {getHideSeries, getLegendItem, getLimitsYAxis, getSeriesItem, getTimeSeries} from "../../../utils/uPlot";

export interface GraphViewProps {
  data?: MetricResult[];
}

const GraphView: FC<GraphViewProps> = ({data = []}) => {
  const { yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItem[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);
  const [valuesLimit, setValuesLimit] = useState<[number, number]>([0, 1]);

  const setLimitsYaxis = (values: number[]) => {
    if (!yaxis.limits.enable || (yaxis.limits.range.every(item => !item))) {
      const limits = getLimitsYAxis(values);
      setValuesLimit(limits);
      graphDispatch({type: "SET_YAXIS_LIMITS", payload: limits});
    }
  };

  const onChangeLegend = (label: string, metaKey: boolean) => {
    setHideSeries(getHideSeries({hideSeries, label, metaKey, series}));
  };

  useEffect(() => {
    const tempTimes: number[] = [];
    const tempValues: number[] = [];
    const tempLegend: LegendItem[] = [];
    const tempSeries: uPlotSeries[] = [];

    data?.forEach(d => {
      const seriesItem = getSeriesItem(d, hideSeries);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem));

      d.values.forEach(v => {
        tempTimes.push(v[0]);
        tempValues.push(+v[1]);
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
      tempLegend.push(getLegendItem(seriesItem));
    });
    setSeries([{}, ...tempSeries]);
    setLegend(tempLegend);
  }, [hideSeries]);

  return <>
    {(data.length > 0)
      ? <div>
        <LineChart data={dataChart} series={series} metrics={data} limits={valuesLimit}/>
        <Legend labels={legend} onChange={onChangeLegend}/>
      </div>
      : <div style={{textAlign: "center"}}>No data to show</div>}
  </>;
};

export default GraphView;