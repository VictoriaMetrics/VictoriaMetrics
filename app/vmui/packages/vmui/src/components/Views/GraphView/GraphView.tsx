import { FC, useEffect, useMemo, useState } from "preact/compat";
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
  getMinMaxBuffer,
  getTimeSeries,
} from "../../../utils/uplot";
import { TimeParams, SeriesItem, LegendItemType } from "../../../types";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { getMathStats } from "../../../utils/math";
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
import { useGraphDispatch } from "../../../state/graph/GraphStateContext";
import { sameTs } from "../../../utils/time";
import { useLocation } from "react-router-dom";
import router from "../../../router";

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
  isPredefinedPanel?: boolean;
  spanGaps?: boolean;
  showAllPoints?: boolean;
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
  isPredefinedPanel,
  spanGaps,
  showAllPoints
}) => {
  const location = useLocation();
  const isRawQuery = useMemo(() => location.pathname === router.rawQuery, [location.pathname]);

  const graphDispatch = useGraphDispatch();

  const [containerRef, containerSize] = useElementSize();

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
    return getSeriesItemContext(data, hideSeries, alias, showAllPoints, isAnomalyView, isRawQuery);
  }, [data, hideSeries, alias, showAllPoints, isAnomalyView, isRawQuery]);

  const setLimitsYaxis = (minVal: number, maxVal: number) => {
    let min = Number.isFinite(minVal) ? minVal : 0;
    let max = Number.isFinite(maxVal) ? maxVal : 1;

    if (min > max) [min, max] = [max, min];

    setYaxisLimits({ "1": isHistogram ? [min, max] : getMinMaxBuffer(min, max) });
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
    const dLen = data.length;

    const tsAnchor = data?.[0]?.values?.[0]?.[0];
    const tsArray: number[] = [];
    const tempLegend = new Array<LegendItemType>(dLen);
    const tempSeries = new Array<uPlotSeries>(dLen + 1);
    tempSeries[0] = {};

    let minVal = Infinity;
    let maxVal = -Infinity;

    for (let i = 0; i < dLen; i++) {
      const d = data[i];
      const seriesItem = getSeriesItem(d, i);
      tempSeries[i + 1] = seriesItem;
      tempLegend[i] = getLegendItem(seriesItem, d.group);

      const vals = d.values;
      for (let j = 0, vLen = vals.length; j < vLen; j++) {
        const v = vals[j];
        if (isRawQuery) tsArray.push(v[0]);
        const num = promValueToNumber(v[1]);
        if (Number.isFinite(num)) {
          if (num < minVal) minVal = num;
          if (num > maxVal) maxVal = num;
        }
      }
    }

    const dpr = window.devicePixelRatio || 1;
    const widthPx = containerSize.width || window.innerWidth || 4096;
    const pixels = Math.max(1, Math.floor(widthPx * Math.max(1, dpr)));

    const timeSeries = isRawQuery
      ? tsArray.sort((a, b) => a - b)
      : getTimeSeries(currentStep, period, pixels, tsAnchor);

    const timeDataSeries: (number | null)[][] = data.map(d => {
      const tsLen = timeSeries.length;
      const results = new Array<number | null>(tsLen);
      const values = d.values;
      const vLen = values.length;

      let j = 0;
      for (let k = 0; k < tsLen; k++) {
        const t = timeSeries[k];
        while (j < vLen && values[j][0] < t) j++;
        let v: number | null = null;
        if (j < vLen && sameTs(values[j][0], t)) {
          const num = promValueToNumber(values[j][1]);
          // Treat special values as nulls in order to satisfy uPlot.
          // Otherwise it may draw unexpected graphs.
          v = Number.isFinite(num) ? num : null;
          // Advance to next value
          j++;
        }
        results[k] = v;
      }

      // // stabilize float numbers
      const { min, max, avg: avgRaw } = getMathStats(results, { min: true, max: true, avg: true });
      const avg = Math.abs(Number(avgRaw));
      const range = getMinMaxBuffer(min, max);
      const rangeStep = Math.abs(range[1] - range[0]);
      const needStabilize = (avg > rangeStep * 1e10) && !isAnomalyView;

      return needStabilize ? results.fill(avg) : results;
    });

    timeDataSeries.unshift(timeSeries);

    const result = isHistogram ? prepareHistogramData(timeDataSeries) : timeDataSeries;
    const legend = prepareAnomalyLegend(tempLegend);

    setLimitsYaxis(minVal, maxVal);
    setDataChart(result as uPlotData);
    setSeries(tempSeries);
    setLegend(legend);
    isAnomalyView && setHideSeries(legend.map(s => s.label || "").slice(1));
  }, [data, timezone, isHistogram, currentStep, isRawQuery]);

  useEffect(() => {
    const dLen = data.length;

    const tempLegend = new Array<LegendItemType>(dLen);
    const tempSeries = new Array<uPlotSeries>(dLen + 1);
    tempSeries[0] = {};

    for (let i = 0; i < dLen; i++) {
      const d = data[i];
      const seriesItem = getSeriesItem(d, i);
      tempSeries[i + 1] = seriesItem;
      tempLegend[i] = getLegendItem(seriesItem, d.group);
    }

    setSeries(tempSeries);
    setLegend(prepareAnomalyLegend(tempLegend));
  }, [hideSeries]);

  const hasTimeData = dataChart[0]?.length > 0;

  useEffect(() => {
    const checkEmptyHistogram = () => {
      if (!isHistogram || !data[1]) {
        return false;
      }

      try {
        const values = (dataChart?.[1]?.[2] || []) as (number | null)[];
        return values.every(v => v === null);
      } catch (e) {
        return false;
      }
    };

    const isEmpty = checkEmptyHistogram();
    graphDispatch({ type: "SET_IS_EMPTY_HISTOGRAM", payload: isEmpty });
  }, [dataChart, isHistogram]);

  return (
    <div
      className={classNames({
        "vm-graph-view": true,
        "vm-graph-view_full-width": fullWidth,
        "vm-graph-view_full-width_mobile": fullWidth && isMobile
      })}
      ref={containerRef}
    >
      {!isHistogram && hasTimeData && (
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
          showAllPoints={isRawQuery ? true : showAllPoints}
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
          isPredefinedPanel={isPredefinedPanel}
        />
      )}
      {isHistogram && showLegend && (
        <LegendHeatmap
          min={yaxis.limits.range[1][0] || 0}
          max={yaxis.limits.range[1][1] || 0}
          legendValue={legendValue}
        />
      )}
    </div>
  );
};

export default GraphView;
