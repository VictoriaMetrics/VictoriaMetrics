import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot, {
  AlignedData as uPlotData,
  Options as uPlotOptions,
} from "uplot";
import { MetricResult } from "../../../../api/types";
import { TimeParams } from "../../../../types";
import "uplot/dist/uPlot.min.css";
import classNames from "classnames";
import { useAppState } from "../../../../state/common/StateContext";
import {
  heatmapPaths,
  handleDestroy,
  getDefaultOptions,
  sizeAxis,
  getAxes,
  setSelect,
} from "../../../../utils/uplot";
import { ElementSize } from "../../../../hooks/useElementSize";
import useReadyChart from "../../../../hooks/uplot/useReadyChart";
import useZoomChart from "../../../../hooks/uplot/useZoomChart";
import usePlotScale from "../../../../hooks/uplot/usePlotScale";
import useHeatmapTooltip from "../../../../hooks/uplot/useHeatmapTooltip";
import ChartTooltipWrapper from "../../ChartTooltip";
import { ChartTooltipProps } from "../../ChartTooltip/ChartTooltip";

export interface HeatmapChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  unit?: string;
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  layoutSize: ElementSize,
  height?: number;
  onChangeLegend: (val: ChartTooltipProps) => void;
}

const HeatmapChart: FC<HeatmapChartProps> = ({
  data,
  metrics = [],
  period,
  unit,
  setPeriod,
  layoutSize,
  height,
  onChangeLegend,
}) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();

  const { xRange, setPlotScale } = usePlotScale({ period, setPeriod });
  const { onReadyChart, isPanning } = useReadyChart(setPlotScale);
  useZoomChart({ uPlotInst, xRange, setPlotScale });
  const {
    stickyTooltips,
    handleUnStick,
    getTooltipProps,
    setCursor,
    resetTooltips
  } = useHeatmapTooltip({ u: uPlotInst, metrics, unit });

  const tooltipProps = useMemo(() => getTooltipProps(), [getTooltipProps]);

  const getHeatmapAxes = () => {
    const baseAxes = getAxes([{}], unit);

    return [
      ...baseAxes,
      {
        scale: "y",
        stroke: baseAxes[0].stroke,
        font: baseAxes[0].font,
        size: sizeAxis,
        splits: metrics.map((m, i) => i),
        values: metrics.map(m => m.metric.vmrange),
      }
    ];
  };
  const options: uPlotOptions = {
    ...getDefaultOptions({ width: layoutSize.width, height }),
    mode: 2,
    series: [
      {},
      {
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-ignore
        paths: heatmapPaths(),
        facets: [
          {
            scale: "x",
            auto: true,
            sorted: 1,
          },
          {
            scale: "y",
            auto: true,
          },
        ],
      },
    ],
    axes: getHeatmapAxes(),
    scales: {
      x: {
        time: true,
      },
      y: {
        log: 2,
        time: false,
        range: (u, initMin, initMax) => [initMin - 1, initMax + 1]
      }
    },
    hooks: {
      ready: [onReadyChart],
      setCursor: [setCursor],
      setSelect: [setSelect(setPlotScale)],
      destroy: [handleDestroy],
    },
  };

  useEffect(() => {
    resetTooltips();
    const isValidData = data[0] === null && Array.isArray(data[1]);
    if (!uPlotRef.current || !isValidData) return;
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    return u.destroy;
  }, [uPlotRef, data, isDarkTheme]);

  useEffect(() => {
    if (!uPlotInst) return;
    uPlotInst.setSize({ width: layoutSize.width || 400, height: height || 500 });
    uPlotInst.redraw();
  }, [height, layoutSize]);

  useEffect(() => {
    onChangeLegend(tooltipProps);
  }, [tooltipProps]);

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
        showTooltip={!!tooltipProps.show}
        tooltipProps={tooltipProps}
        stickyTooltips={stickyTooltips}
        handleUnStick={handleUnStick}
      />
    </div>
  );
};

export default HeatmapChart;
