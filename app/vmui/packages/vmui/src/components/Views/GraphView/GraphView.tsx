import React, { FC, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import { MetricResult } from "../../../api/types";
import LineChart from "../../Chart/Line/LineChart/LineChart";
import { AlignedData as uPlotData, Series as uPlotSeries } from "uplot";
import Legend from "../../Chart/Line/Legend/Legend";
import LegendHeatmap from "../../Chart/Heatmap/LegendHeatmap/LegendHeatmap";
import { getHideSeries, getLegendItem, getSeriesItemContext } from "../../../utils/uplot/series";
import { getLimitsYAxis, getMinMaxBuffer, getTimeSeries } from "../../../utils/uplot/axes";
import { LegendItemType } from "../../../utils/uplot/types";
import { TimeParams } from "../../../types";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { getAvgFromArray, getMaxFromArray, getMinFromArray } from "../../../utils/math";
import classNames from "classnames";
import { useTimeState } from "../../../state/time/TimeStateContext";
import HeatmapChart from "../../Chart/Heatmap/HeatmapChart/HeatmapChart";
import "./style.scss";
import { promValueToNumber } from "../../../utils/metric";
import { normalizeData } from "../../../utils/uplot/heatmap";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

export interface GraphViewProps {
  data?: MetricResult[];
  period: TimeParams;
  customStep: string;
  query: string[];
  alias?: string[],
  yaxis: YaxisState;
  unit?: string;
  showLegend?: boolean;
  setYaxisLimits: (val: AxisRange) => void
  setPeriod: ({ from, to }: {from: Date, to: Date}) => void
  fullWidth?: boolean
  height?: number
  isHistogram?: boolean
}

const GraphView: FC<GraphViewProps> = ({
  data: dataRaw = [],
  period,
  customStep,
  query,
  yaxis,
  unit,
  showLegend= true,
  setYaxisLimits,
  setPeriod,
  alias = [],
  fullWidth = true,
  height,
  isHistogram
}) => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();
  const currentStep = useMemo(() => customStep || period.step || "1s", [period.step, customStep]);

  const data = useMemo(() => normalizeData(dataRaw, isHistogram), [isHistogram, dataRaw]);
  const getSeriesItem = useCallback(getSeriesItemContext(), [data]);

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItemType[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);
  const [legendValue, setLegendValue] = useState(0);

  const setLimitsYaxis = (values: {[key: string]: number[]}) => {
    const limits = getLimitsYAxis(values, !isHistogram);
    setYaxisLimits(limits);
  };

  const onChangeLegend = (legend: LegendItemType, metaKey: boolean) => {
    setHideSeries(getHideSeries({ hideSeries, legend, metaKey, series }));
  };

  const handleChangeLegend = (val: number) => {
    setLegendValue(val);
  };

  const prepareHistogramData = (data: (number | null)[][]) => {
    const values = data.slice(1, data.length);
    const xs: (number | null | undefined)[] = [];
    const counts: (number | null | undefined)[] = [];

    values.forEach((arr, indexRow) => {
      arr.forEach((v, indexValue) => {
        const targetIndex = (indexValue * values.length) + indexRow;
        counts[targetIndex] = v;
      });
    });

    data[0].forEach(t => {
      const arr = new Array(values.length).fill(t);
      xs.push(...arr);
    });

    const ys = new Array(xs.length).fill(0).map((n, i) => i % (values.length));

    return [null, [xs, ys, counts]];
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
    const result = isHistogram ? prepareHistogramData(timeDataSeries) : timeDataSeries;
    setDataChart(result as uPlotData);
    setSeries(tempSeries);
    setLegend(tempLegend);
  }, [data, timezone, isHistogram]);

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
        "vm-graph-view_full-width": fullWidth,
        "vm-graph-view_full-width_mobile": fullWidth && isMobile
      })}
      ref={containerRef}
    >
      {containerRef?.current && !isHistogram && (
        <LineChart
          data={dataChart}
          series={series}
          metrics={data}
          period={period}
          yaxis={yaxis}
          unit={unit}
          setPeriod={setPeriod}
          container={containerRef?.current}
          height={height}
        />
      )}
      {containerRef?.current && isHistogram && (
        <HeatmapChart
          data={dataChart}
          metrics={data}
          period={period}
          yaxis={yaxis}
          unit={unit}
          setPeriod={setPeriod}
          container={containerRef?.current}
          height={height}
          onChangeLegend={handleChangeLegend}
        />
      )}
      {!isHistogram && showLegend && (
        <Legend
          labels={legend}
          query={query}
          onChange={onChangeLegend}
        />
      )}
      {isHistogram && showLegend && (
        <LegendHeatmap
          min={yaxis.limits.range[1][0] || 0}
          max={yaxis.limits.range[1][1] || 0}
          value={legendValue}
        />
      )}
    </div>
  );
};

export default GraphView;
