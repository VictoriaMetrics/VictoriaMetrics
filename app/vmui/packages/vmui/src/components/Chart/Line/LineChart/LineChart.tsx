import React, { FC, useCallback, useEffect, useRef, useState } from "preact/compat";
import uPlot, {
  AlignedData as uPlotData,
  Options as uPlotOptions,
  Series as uPlotSeries,
} from "uplot";
import { defaultOptions } from "../../../../utils/uplot/helpers";
import { dragChart } from "../../../../utils/uplot/events";
import { getAxes } from "../../../../utils/uplot/axes";
import { MetricResult } from "../../../../api/types";
import { dateFromSeconds, formatDateForNativeInput, limitsDurations } from "../../../../utils/time";
import { TimeParams } from "../../../../types";
import { YaxisState } from "../../../../state/graph/reducer";
import "uplot/dist/uPlot.min.css";
import "./style.scss";
import classNames from "classnames";
import ChartTooltip, { ChartTooltipProps } from "../ChartTooltip/ChartTooltip";
import dayjs from "dayjs";
import { useAppState } from "../../../../state/common/StateContext";
import { SeriesItem } from "../../../../utils/uplot/series";
import { ElementSize } from "../../../../hooks/useElementSize";
import useEventListener from "../../../../hooks/useEventListener";
import { getRangeX, getRangeY, getScales } from "../../../../utils/uplot/scales";

export interface LineChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  yaxis: YaxisState;
  series: uPlotSeries[];
  unit?: string;
  setPeriod: ({ from, to }: {from: Date, to: Date}) => void;
  layoutSize: ElementSize;
  height?: number;
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
  height
}) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [isPanning, setPanning] = useState(false);
  const [xRange, setXRange] = useState({ min: period.start, max: period.end });
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const [startTouchDistance, setStartTouchDistance] = useState(0);

  const [showTooltip, setShowTooltip] = useState(false);
  const [tooltipIdx, setTooltipIdx] = useState({ seriesIdx: -1, dataIdx: -1 });
  const [tooltipOffset, setTooltipOffset] = useState({ left: 0, top: 0 });
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipProps[]>([]);

  const setPlotScale = ({ min, max }: { min: number, max: number }) => {
    const delta = (max - min) * 1000;
    if ((delta < limitsDurations.min) || (delta > limitsDurations.max)) return;
    setXRange({ min, max });
    setPeriod({
      from: dayjs(min * 1000).toDate(),
      to: dayjs(max * 1000).toDate()
    });
  };

  const onReadyChart = (u: uPlot): void => {
    const factor = 0.9;
    setTooltipOffset({
      left: parseFloat(u.over.style.left),
      top: parseFloat(u.over.style.top)
    });

    u.over.addEventListener("mousedown", e => {
      const { ctrlKey, metaKey, button } = e;
      const leftClick = button === 0;
      const leftClickWithMeta = leftClick && (ctrlKey || metaKey);
      if (leftClickWithMeta) {
        // drag pan
        dragChart({ u, e, setPanning, setPlotScale, factor });
      }
    });

    u.over.addEventListener("touchstart", e => {
      dragChart({ u, e, setPanning, setPlotScale, factor });
    });

    u.over.addEventListener("wheel", e => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      const { width } = u.over.getBoundingClientRect();
      const zoomPos = u.cursor.left && u.cursor.left > 0 ? u.cursor.left : 0;
      const xVal = u.posToVal(zoomPos, "x");
      const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
      const nxRange = e.deltaY < 0 ? oxRange * factor : oxRange / factor;
      const min = xVal - (zoomPos / width) * nxRange;
      const max = min + nxRange;
      u.batch(() => setPlotScale({ min, max }));
    });
  };

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    const { target, ctrlKey, metaKey, key } = e;
    const isInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement;
    if (!uPlotInst || isInput) return;
    const minus = key === "-";
    const plus = key === "+" || key === "=";
    if ((minus || plus) && !(ctrlKey || metaKey)) {
      e.preventDefault();
      const factor = (xRange.max - xRange.min) / 10 * (plus ? 1 : -1);
      setPlotScale({
        min: xRange.min + factor,
        max: xRange.max - factor
      });
    }
  }, [uPlotInst, xRange]);

  const getChartProps = useCallback(() => {
    const { seriesIdx, dataIdx } = tooltipIdx;
    const id = `${seriesIdx}_${dataIdx}`;
    const metricItem = metrics[seriesIdx-1];
    const seriesItem = series[seriesIdx] as SeriesItem;

    const groups = new Set(metrics.map(m => m.group));
    const showQueryNum = groups.size > 1;

    return {
      id,
      unit,
      seriesItem,
      metricItem,
      tooltipIdx,
      tooltipOffset,
      showQueryNum,
    };
  }, [uPlotInst, metrics, series, tooltipIdx, tooltipOffset, unit]);

  const handleClick = useCallback(() => {
    if (!showTooltip) return;
    const props = getChartProps();
    if (!stickyTooltips.find(t => t.id === props.id)) {
      setStickyToolTips(prev => [...prev, props as ChartTooltipProps]);
    }
  }, [getChartProps, stickyTooltips, showTooltip]);

  const handleUnStick = (id: string) => {
    setStickyToolTips(prev => prev.filter(t => t.id !== id));
  };

  const setCursor = (u: uPlot) => {
    const dataIdx = u.cursor.idx ?? -1;
    setTooltipIdx(prev => ({ ...prev, dataIdx }));
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    const seriesIdx = sidx ?? -1;
    setTooltipIdx(prev => ({ ...prev, seriesIdx }));
  };

  const addSeries = (u: uPlot, series: uPlotSeries[]) => {
    series.forEach((s) => {
      u.addSeries(s);
    });
  };

  const delSeries = (u: uPlot) => {
    for (let i = u.series.length - 1; i >= 0; i--) {
      u.delSeries(i);
    }
  };

  const delHooks = (u: uPlot) => {
    Object.keys(u.hooks).forEach(hook => {
      u.hooks[hook as keyof uPlot.Hooks.Arrays] = [];
    });
  };

  const handleDestroy = (u: uPlot) => {
    delSeries(u);
    delHooks(u);
    u.setData([]);
  };

  const setSelect = (u: uPlot) => {
    const min = u.posToVal(u.select.left, "x");
    const max = u.posToVal(u.select.left + u.select.width, "x");
    setPlotScale({ min, max });
  };

  const options: uPlotOptions = {
    ...defaultOptions,
    tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
    series,
    axes: getAxes( [{}, { scale: "1" }], unit),
    scales: getScales(yaxis, xRange),
    width: layoutSize.width || 400,
    height: height || 500,
    hooks: {
      ready: [onReadyChart],
      setSeries: [seriesFocus],
      setCursor: [setCursor],
      setSelect: [setSelect],
      destroy: [handleDestroy],
    },
  };

  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 2) return;
    e.preventDefault();

    const dx = e.touches[0].clientX - e.touches[1].clientX;
    const dy = e.touches[0].clientY - e.touches[1].clientY;
    setStartTouchDistance(Math.sqrt(dx * dx + dy * dy));
  };

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (e.touches.length !== 2 || !uPlotInst) return;
    e.preventDefault();

    const dx = e.touches[0].clientX - e.touches[1].clientX;
    const dy = e.touches[0].clientY - e.touches[1].clientY;
    const endTouchDistance = Math.sqrt(dx * dx + dy * dy);
    const diffDistance = startTouchDistance - endTouchDistance;

    const max = (uPlotInst.scales.x.max || xRange.max);
    const min = (uPlotInst.scales.x.min || xRange.min);
    const dur = max - min;
    const dir = (diffDistance > 0 ? -1 : 1);

    const zoomFactor = dur / 50 * dir;
    uPlotInst.batch(() => setPlotScale({
      min: min + zoomFactor,
      max: max - zoomFactor
    }));
  }, [uPlotInst, startTouchDistance, xRange]);

  useEffect(() => {
    setXRange({ min: period.start, max: period.end });
  }, [period]);

  useEffect(() => {
    setStickyToolTips([]);
    setTooltipIdx({ seriesIdx: -1, dataIdx: -1 });
    if (!uPlotRef.current) return;
    if (uPlotInst) uPlotInst.destroy();
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    setXRange({ min: period.start, max: period.end });
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
    addSeries(uPlotInst, series);
    uPlotInst.redraw();
  }, [series]);

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

  useEffect(() => {
    setShowTooltip(tooltipIdx.dataIdx !== -1 && tooltipIdx.seriesIdx !== -1);
  }, [tooltipIdx]);

  useEventListener("click", handleClick);
  useEventListener("keydown", handleKeyDown);
  useEventListener("touchmove", handleTouchMove);
  useEventListener("touchstart", handleTouchStart);

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
      {uPlotInst && showTooltip && (
        <ChartTooltip
          {...getChartProps()}
          u={uPlotInst}
        />
      )}

      {uPlotInst && stickyTooltips.map(t => (
        <ChartTooltip
          {...t}
          isSticky
          u={uPlotInst}
          key={t.id}
          onClose={handleUnStick}
        />
      ))}
    </div>
  );
};

export default LineChart;
