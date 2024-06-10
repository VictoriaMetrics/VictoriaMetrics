import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { MetricResult } from "../../../api/types";
import LineChart from "../../Chart/Line/LineChart/LineChart";
import { AlignedData as uPlotData, Series as uPlotSeries } from "uplot";
import Legend from "../../Chart/Line/Legend/Legend";
import LegendHeatmap from "../../Chart/Heatmap/LegendHeatmap/LegendHeatmap";
import {
  getHideSeries,
  getLegendItem,
  getSeriesItemContext,
  normalizeData,
  getLimitsYAxis,
  getMinMaxBuffer,
  getTimeSeries,
} from "../../../utils/uplot";
import { TimeParams, SeriesItem, LegendItemType } from "../../../types";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { getAvgFromArray, getMaxFromArray, getMinFromArray } from "../../../utils/math";
import classNames from "classnames";
import { useTimeState } from "../../../state/time/TimeStateContext";
import HeatmapChart from "../../Chart/Heatmap/HeatmapChart/HeatmapChart";
import "./style.scss";
import { promValueToNumber } from "../../../utils/metric";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useElementSize from "../../../hooks/useElementSize";
import { ChartTooltipProps } from "../../Chart/ChartTooltip/ChartTooltip";
import LegendAnomaly from "../../Chart/Line/LegendAnomaly/LegendAnomaly";
import { groupByMultipleKeys } from "../../../utils/array";

export interface GraphViewProps {
  data?: MetricResult[];
  period: TimeParams;
  customStep: string;
  query: string[];
  alias?: string[],
  yaxis: YaxisState;
  unit?: string;
  showLegend?: boolean;
  setYaxisLimits: (val: AxisRange) => void;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  fullWidth?: boolean;
  height?: number;
  isHistogram?: boolean;
  isAnomalyView?: boolean;
  spanGaps?: boolean;
}

const GraphView: FC<GraphViewProps> = ({
  data: dataRaw = [],
  period,
  customStep,
  query,
  yaxis,
  unit,
  showLegend = true,
  setYaxisLimits,
  setPeriod,
  alias = [],
  fullWidth = true,
  height,
  isHistogram,
  isAnomalyView,
  spanGaps
}) => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();
  const currentStep = useMemo(() => customStep || period.step || "1s", [period.step, customStep]);

  const data = useMemo(() => normalizeData(dataRaw, isHistogram), [isHistogram, dataRaw]);

  const [dataChart, setDataChart] = useState<uPlotData>([[]]);
  const [series, setSeries] = useState<uPlotSeries[]>([]);
  const [legend, setLegend] = useState<LegendItemType[]>([]);
  const [hideSeries, setHideSeries] = useState<string[]>([]);
  const [legendValue, setLegendValue] = useState<ChartTooltipProps | null>(null);

  const getSeriesItem = useMemo(() => {
    return getSeriesItemContext(data, hideSeries, alias, isAnomalyView);
  }, [data, hideSeries, alias, isAnomalyView]);

  const setLimitsYaxis = (values: { [key: string]: number[] }) => {
    const limits = getLimitsYAxis(values, !isHistogram);
    setYaxisLimits(limits);
  };

  const onChangeLegend = (legend: LegendItemType, metaKey: boolean) => {
    setHideSeries(getHideSeries({ hideSeries, legend, metaKey, series, isAnomalyView }));
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

  const prepareAnomalyLegend = (legend: LegendItemType[]): LegendItemType[] => {
    if (!isAnomalyView) return legend;

    // For vmanomaly: Only select the first series per group (due to API specs) and clear __name__ in freeFormFields.
    const grouped = groupByMultipleKeys(legend, ["group", "label"]);
    return grouped.map((group) => {
      const firstEl = group.values[0];
      return {
        ...firstEl,
        freeFormFields: { ...firstEl.freeFormFields, __name__: "" }
      };
    });
  };

  useEffect(() => {
    const tempTimes: number[] = [];
    const tempValues: { [key: string]: number[] } = {};
    const tempLegend: LegendItemType[] = [];
    const tempSeries: uPlotSeries[] = [{}];

    data?.forEach((d, i) => {
      const seriesItem = getSeriesItem(d, i);

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

      return (avg > rangeStep * 1e10) && !isAnomalyView ? results.map(() => avg) : results;
    });
    timeDataSeries.unshift(timeSeries);
    setLimitsYaxis(tempValues);
    const result = isHistogram ? prepareHistogramData(timeDataSeries) : timeDataSeries;
    setDataChart(result as uPlotData);
    setSeries(tempSeries);
    const legend = prepareAnomalyLegend(tempLegend);
    setLegend(legend);
    if (isAnomalyView) {
      setHideSeries(legend.map(s => s.label || "").slice(1));
    }
  }, [data, timezone, isHistogram]);

  useEffect(() => {
    const tempLegend: LegendItemType[] = [];
    const tempSeries: uPlotSeries[] = [{}];
    data?.forEach((d, i) => {
      const seriesItem = getSeriesItem(d, i);
      tempSeries.push(seriesItem);
      tempLegend.push(getLegendItem(seriesItem, d.group));
    });
    setSeries(tempSeries);
    setLegend(prepareAnomalyLegend(tempLegend));
  }, [hideSeries]);

  const [containerRef, containerSize] = useElementSize();

  return (
    <div
      className={classNames({
        "vm-graph-view": true,
        "vm-graph-view_full-width": fullWidth,
        "vm-graph-view_full-width_mobile": fullWidth && isMobile
      })}
      ref={containerRef}
    >
      {!isHistogram && (
        <LineChart
          data={dataChart}
          series={series}
          metrics={data}
          period={period}
          yaxis={yaxis}
          unit={unit}
          setPeriod={setPeriod}
          layoutSize={containerSize}
          height={height}
          isAnomalyView={isAnomalyView}
          spanGaps={spanGaps}
        />
      )}
      {isHistogram && (
        <HeatmapChart
          data={dataChart}
          metrics={data}
          period={period}
          unit={unit}
          setPeriod={setPeriod}
          layoutSize={containerSize}
          height={height}
          onChangeLegend={setLegendValue}
        />
      )}
      {isAnomalyView && showLegend && (<LegendAnomaly series={series as SeriesItem[]}/>)}
      {!isHistogram && showLegend && (
        <Legend
          labels={legend}
          query={query}
          isAnomalyView={isAnomalyView}
          onChange={onChangeLegend}
        />
      )}
      {isHistogram && showLegend && (
        <LegendHeatmap
          series={series as SeriesItem[]}
          min={yaxis.limits.range[1][0] || 0}
          max={yaxis.limits.range[1][1] || 0}
          legendValue={legendValue}
        />
      )}
    </div>
  );
};

export default GraphView;
