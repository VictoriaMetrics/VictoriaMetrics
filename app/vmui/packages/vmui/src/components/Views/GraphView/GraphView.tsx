import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { MetricResult } from "../../../api/types";
import LineChart from "../../Chart/LineChart/LineChart";
import { AlignedData as uPlotData, Series as uPlotSeries } from "uplot";
import Legend from "../../Chart/Legend/Legend";
import { getHideSeries, getLegendItem, getSeriesItem } from "../../../utils/uplot/series";
import { getLimitsYAxis, getMinMaxBuffer, getTimeSeries } from "../../../utils/uplot/axes";
import { LegendItemType } from "../../../utils/uplot/types";
import { TimeParams } from "../../../types";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { getAvgFromArray, getMaxFromArray, getMinFromArray } from "../../../utils/math";
import classNames from "classnames";
import { useTimeState } from "../../../state/time/TimeStateContext";
import "./style.scss";

export interface GraphViewProps {
  data?: MetricResult[];
  period: TimeParams;
  customStep: number;
  query: string[];
  alias?: string[],
  yaxis: YaxisState;
  unit?: string;
  showLegend?: boolean;
  setYaxisLimits: (val: AxisRange) => void
  setPeriod: ({ from, to }: {from: Date, to: Date}) => void
  fullWidth?: boolean
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
  setPeriod,
  alias = [],
  fullWidth = true
}) => {
  const { timezone } = useTimeState();
  const currentStep = useMemo(() => customStep || period.step || 1, [period.step, customStep]);

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItemType[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);

  const setLimitsYaxis = (values: {[key: string]: number[]}) => {
    const limits = getLimitsYAxis(values);
    setYaxisLimits(limits);
  };

  const onChangeLegend = (legend: LegendItemType, metaKey: boolean) => {
    setHideSeries(getHideSeries({ hideSeries, legend, metaKey, series }));
  };

  useEffect(() => {
    const tempTimes: number[] = [];
    const tempValues: {[key: string]: number[]} = {};
    const tempLegend: LegendItemType[] = [];
    const tempSeries: uPlotSeries[] = [{}];

    data?.forEach((d) => {
      const seriesItem = getSeriesItem(d, hideSeries, alias);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem, d.group));
      const tmpValues = tempValues[d.group] || [];
      for (const v of d.values) {
        tempTimes.push(v[0]);
        tmpValues.push(promValueToNumber(v[1]));
      }
      tempValues[d.group] = tmpValues;
    });

    const timeSeries = getTimeSeries(tempTimes, currentStep, period);
    const timeDataSeries = data.map(d => {
      const results = [];
      const values = d.values;
      const length = values.length;
      let j = 0;
      for (const t of timeSeries) {
        while (j < length && values[j][0] < t) j++;
        let v = null;
        if (j < length && values[j][0] == t) {
          v = promValueToNumber(values[j][1]);
          if (!Number.isFinite(v)) {
            // Treat special values as nulls in order to satisfy uPlot.
            // Otherwise it may draw unexpected graphs.
            v = null;
          }
        }
        results.push(v);
      }

      // stabilize float numbers
      const resultAsNumber = results.filter(s => s !== null) as number[];
      const avg = Math.abs(getAvgFromArray(resultAsNumber));
      const range = getMinMaxBuffer(getMinFromArray(resultAsNumber), getMaxFromArray(resultAsNumber));
      const rangeStep = Math.abs(range[1] - range[0]);

      return (avg > rangeStep * 1e10) ? results.map(() => avg) : results;
    });
    timeDataSeries.unshift(timeSeries);
    setLimitsYaxis(tempValues);
    setDataChart(timeDataSeries as uPlotData);
    setSeries(tempSeries);
    setLegend(tempLegend);
  }, [data, timezone]);

  useEffect(() => {
    const tempLegend: LegendItemType[] = [];
    const tempSeries: uPlotSeries[] = [{}];
    data?.forEach(d => {
      const seriesItem = getSeriesItem(d, hideSeries, alias);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem, d.group));
    });
    setSeries(tempSeries);
    setLegend(tempLegend);
  }, [hideSeries]);

  const containerRef = useRef<HTMLDivElement>(null);

  return (
    <div
      className={classNames({
        "vm-graph-view": true,
        "vm-graph-view_full-width": fullWidth
      })}
      ref={containerRef}
    >
      {containerRef?.current &&
        <LineChart
          data={dataChart}
          series={series}
          metrics={data}
          period={period}
          yaxis={yaxis}
          unit={unit}
          setPeriod={setPeriod}
          container={containerRef?.current}
        />}
      {showLegend && <Legend
        labels={legend}
        query={query}
        onChange={onChangeLegend}
      />}
    </div>
  );
};

export default GraphView;
