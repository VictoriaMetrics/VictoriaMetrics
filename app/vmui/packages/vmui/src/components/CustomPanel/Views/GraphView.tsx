import React, {FC, useEffect, useMemo, useRef, useState} from "preact/compat";
import {MetricResult} from "../../../api/types";
import LineChart from "../../LineChart/LineChart";
import {AlignedData as uPlotData, Series as uPlotSeries} from "uplot";
import Legend from "../../Legend/Legend";
import {getHideSeries, getLegendItem, getSeriesItem} from "../../../utils/uplot/series";
import {getLimitsYAxis, getTimeSeries} from "../../../utils/uplot/axes";
import {LegendItem} from "../../../utils/uplot/types";
import {TimeParams} from "../../../types";
import {AxisRange, CustomStep, YaxisState} from "../../../state/graph/reducer";

export interface GraphViewProps {
  data?: MetricResult[];
  period: TimeParams;
  customStep: CustomStep;
  query: string[];
  yaxis: YaxisState;
  unit?: string;
  showLegend?: boolean;
  setYaxisLimits: (val: AxisRange) => void
  setPeriod: ({from, to}: {from: Date, to: Date}) => void
}

const promValueToNumber = (s: string): number => {
  // See https://prometheus.io/docs/prometheus/latest/querying/api/#expression-query-result-formats
  switch (s) {
    case "NaN":
      return NaN;
    case "Inf":
    case "+Inf":
      return Infinity;
    case "-Inf":
      return -Infinity;
    default:
      return parseFloat(s);
  }
};

const GraphView: FC<GraphViewProps> = ({
  data = [],
  period,
  customStep,
  query,
  yaxis,
  unit,
  showLegend= true,
  setYaxisLimits,
  setPeriod
}) => {
  const currentStep = useMemo(() => customStep.enable ? customStep.value : period.step || 1, [period.step, customStep]);

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItem[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);

  const setLimitsYaxis = (values: {[key: string]: number[]}) => {
    const limits = getLimitsYAxis(values);
    setYaxisLimits(limits);
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
      let tmpValues = tempValues[d.group];
      if (!tmpValues) {
        tmpValues = [];
      }
      for (const v of d.values) {
        tempTimes.push(v[0]);
        tmpValues.push(promValueToNumber(v[1]));
      }
      tempValues[d.group] = tmpValues;
    });

    const timeSeries = getTimeSeries(tempTimes, currentStep, period);
    setDataChart([timeSeries, ...data.map(d => {
      const results = [];
      const values = d.values;
      let j = 0;
      for (const t of timeSeries) {
        while (j < values.length && values[j][0] < t) j++;
        let v = null;
        if (j < values.length && values[j][0] == t) {
          v = promValueToNumber(values[j][1]);
          if (!Number.isFinite(v)) {
            // Treat special values as nulls in order to satisfy uPlot.
            // Otherwise it may draw unexpected graphs.
            v = null;
          }
        }
        results.push(v);
      }
      return results;
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

  const containerRef = useRef<HTMLDivElement>(null);

  return <>
    <div style={{width: "100%"}} ref={containerRef}>
      {containerRef?.current &&
          <LineChart data={dataChart} series={series} metrics={data} period={period} yaxis={yaxis} unit={unit}
            setPeriod={setPeriod} container={containerRef?.current}/>}
      {showLegend && <Legend labels={legend} query={query} onChange={onChangeLegend}/>}
    </div>
  </>;
};

export default GraphView;
