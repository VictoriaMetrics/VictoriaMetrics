import React, { FC, useEffect, useRef, useState } from "preact/compat";
import uPlot, {
  AlignedData as uPlotData,
  Options as uPlotOptions,
  Series as uPlotSeries,
} from "uplot";
import {
  addSeries,
  delSeries,
  getAxes,
  getDefaultOptions,
  getRangeX,
  getRangeY,
  getScales,
  handleDestroy,
  setBand,
  setSelect
} from "../../../../utils/uplot";
import { MetricResult } from "../../../../api/types";
import { TimeParams } from "../../../../types";
import { YaxisState } from "../../../../state/graph/reducer";
import "uplot/dist/uPlot.min.css";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../../../state/common/StateContext";
import { ElementSize } from "../../../../hooks/useElementSize";
import useReadyChart from "../../../../hooks/uplot/useReadyChart";
import useZoomChart from "../../../../hooks/uplot/useZoomChart";
import usePlotScale from "../../../../hooks/uplot/usePlotScale";
import useLineTooltip from "../../../../hooks/uplot/useLineTooltip";
import ChartTooltipWrapper from "../../ChartTooltip";

export interface LineChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  yaxis: YaxisState;
  series: uPlotSeries[];
  unit?: string;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  layoutSize: ElementSize;
  height?: number;
  isAnomalyView?: boolean;
  spanGaps?: boolean;
}

const LineChart: FC<LineChartProps> = ({
  data,
  series,
  metrics = [],
  period,
  yaxis,
  unit,
  setPeriod,
  layoutSize,
  height,
  isAnomalyView,
  spanGaps = false
}) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();

  const { xRange, setPlotScale } = usePlotScale({ period, setPeriod });
  const { onReadyChart, isPanning } = useReadyChart(setPlotScale);
  useZoomChart({ uPlotInst, xRange, setPlotScale });
  const {
    showTooltip,
    stickyTooltips,
    handleUnStick,
    getTooltipProps,
    seriesFocus,
    setCursor,
    resetTooltips
  } = useLineTooltip({ u: uPlotInst, metrics, series, unit, isAnomalyView });

  const options: uPlotOptions = {
    ...getDefaultOptions({ width: layoutSize.width, height }),
    series,
    axes: getAxes([{}, { scale: "1" }], unit),
    scales: getScales(yaxis, xRange),
    hooks: {
      ready: [onReadyChart],
      setSeries: [seriesFocus],
      setCursor: [setCursor],
      setSelect: [setSelect(setPlotScale)],
      destroy: [handleDestroy],
    },
    bands: []
  };

  useEffect(() => {
    resetTooltips();
    if (!uPlotRef.current) return;
    if (uPlotInst) uPlotInst.destroy();
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    return u.destroy;
  }, [uPlotRef, isDarkTheme]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.setData(data);
    uPlotInst.redraw();
  }, [data]);

  useEffect(() => {
    if (!uPlotInst) return;
    delSeries(uPlotInst);
    addSeries(uPlotInst, series, spanGaps);
    setBand(uPlotInst, series);
    uPlotInst.redraw();
  }, [series, spanGaps]);

  useEffect(() => {
    if (!uPlotInst) return;
    Object.keys(yaxis.limits.range).forEach(axis => {
      if (!uPlotInst.scales[axis]) return;
      uPlotInst.scales[axis].range = (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis, yaxis);
    });
    uPlotInst.redraw();
  }, [yaxis]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.scales.x.range = () => getRangeX(xRange);
    uPlotInst.redraw();
  }, [xRange]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.setSize({ width: layoutSize.width || 400, height: height || 500 });
    uPlotInst.redraw();
  }, [height, layoutSize]);

  return (
    <div
      className={classNames({
        "vm-line-chart": true,
        "vm-line-chart_panning": isPanning
      })}
      style={{
        minWidth: `${layoutSize.width || 400}px`,
        minHeight: `${height || 500}px`
      }}
    >
      <div
        className="vm-line-chart__u-plot"
        ref={uPlotRef}
      />
      <ChartTooltipWrapper
        showTooltip={showTooltip}
        tooltipProps={getTooltipProps()}
        stickyTooltips={stickyTooltips}
        handleUnStick={handleUnStick}
      />
    </div>
  );
};

export default LineChart;
