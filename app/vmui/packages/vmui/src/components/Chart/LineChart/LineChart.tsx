import React, { FC, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot, {
  AlignedData as uPlotData,
  Options as uPlotOptions,
  Series as uPlotSeries,
  Range,
  Scales,
  Scale,
} from "uplot";
import { defaultOptions } from "../../../utils/uplot/helpers";
import { dragChart } from "../../../utils/uplot/events";
import { getAxes, getMinMaxBuffer } from "../../../utils/uplot/axes";
import { MetricResult } from "../../../api/types";
import { dateFromSeconds, formatDateForNativeInput, limitsDurations } from "../../../utils/time";
import throttle from "lodash.throttle";
import useResize from "../../../hooks/useResize";
import { TimeParams } from "../../../types";
import { YaxisState } from "../../../state/graph/reducer";
import "uplot/dist/uPlot.min.css";
import "./style.scss";
import classNames from "classnames";
import ChartTooltip, { ChartTooltipProps } from "../ChartTooltip/ChartTooltip";
import dayjs from "dayjs";
import { useAppState } from "../../../state/common/StateContext";

export interface LineChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  yaxis: YaxisState;
  series: uPlotSeries[];
  unit?: string;
  setPeriod: ({ from, to }: {from: Date, to: Date}) => void;
  container: HTMLDivElement | null;
  height?: number;
}

enum typeChartUpdate {xRange = "xRange", yRange = "yRange", data = "data"}

const LineChart: FC<LineChartProps> = ({
  data,
  series,
  metrics = [],
  period,
  yaxis,
  unit,
  setPeriod,
  container,
  height
}) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [isPanning, setPanning] = useState(false);
  const [xRange, setXRange] = useState({ min: period.start, max: period.end });
  const [yRange, setYRange] = useState([0, 1]);
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const layoutSize = useResize(container);

  const [showTooltip, setShowTooltip] = useState(false);
  const [tooltipIdx, setTooltipIdx] = useState({ seriesIdx: -1, dataIdx: -1 });
  const [tooltipOffset, setTooltipOffset] = useState({ left: 0, top: 0 });
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipProps[]>([]);
  const tooltipId = useMemo(() => `${tooltipIdx.seriesIdx}_${tooltipIdx.dataIdx}`, [tooltipIdx]);

  const setScale = ({ min, max }: { min: number, max: number }): void => {
    setPeriod({
      from: dayjs(min * 1000).toDate(),
      to: dayjs(max * 1000).toDate()
    });
  };
  const throttledSetScale = useCallback(throttle(setScale, 500), []);
  const setPlotScale = ({ u, min, max }: { u: uPlot, min: number, max: number }) => {
    const delta = (max - min) * 1000;
    if ((delta < limitsDurations.min) || (delta > limitsDurations.max)) return;
    u.setScale("x", { min, max });
    setXRange({ min, max });
    throttledSetScale({ min, max });
  };

  const onReadyChart = (u: uPlot) => {
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
      u.batch(() => setPlotScale({ u, min, max }));
    });
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const { target, ctrlKey, metaKey, key } = e;
    const isInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement;
    if (!uPlotInst || isInput) return;
    const minus = key === "-";
    const plus = key === "+" || key === "=";
    if ((minus || plus) && !(ctrlKey || metaKey)) {
      e.preventDefault();
      const factor = (xRange.max - xRange.min) / 10 * (plus ? 1 : -1);
      setPlotScale({
        u: uPlotInst,
        min: xRange.min + factor,
        max: xRange.max - factor
      });
    }
  };

  const handleClick = () => {
    const id = `${tooltipIdx.seriesIdx}_${tooltipIdx.dataIdx}`;
    const props = {
      id,
      unit,
      series,
      metrics,
      yRange,
      tooltipIdx,
      tooltipOffset,
    };

    if (!stickyTooltips.find(t => t.id === id)) {
      const tooltipProps = JSON.parse(JSON.stringify(props));
      setStickyToolTips(prev => [...prev, tooltipProps]);
    }
  };

  const handleUnStick = (id:string) => {
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

  const getRangeX = (): Range.MinMax => [xRange.min, xRange.max];

  const getRangeY = (u: uPlot, min = 0, max = 1, axis: string): Range.MinMax => {
    if (axis == "1") {
      setYRange([min, max]);
    }
    if (yaxis.limits.enable) return yaxis.limits.range[axis];
    return getMinMaxBuffer(min, max);
  };

  const getScales = (): Scales => {
    const scales: { [key: string]: { range: Scale.Range } } = { x: { range: getRangeX } };
    const ranges = Object.keys(yaxis.limits.range);
    (ranges.length ? ranges : ["1"]).forEach(axis => {
      scales[axis] = { range: (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis) };
    });
    return scales;
  };

  const options: uPlotOptions = {
    ...defaultOptions,
    tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
    series,
    axes: getAxes( [{}, { scale: "1" }], unit),
    scales: { ...getScales() },
    width: layoutSize.width || 400,
    height: height || 500,
    plugins: [{ hooks: { ready: onReadyChart, setCursor, setSeries: seriesFocus } }],
    hooks: {
      setSelect: [
        (u) => {
          const min = u.posToVal(u.select.left, "x");
          const max = u.posToVal(u.select.left + u.select.width, "x");
          setPlotScale({ u, min, max });
        }
      ]
    }
  };

  const updateChart = (type: typeChartUpdate): void => {
    if (!uPlotInst) return;
    switch (type) {
      case typeChartUpdate.xRange:
        uPlotInst.scales.x.range = getRangeX;
        break;
      case typeChartUpdate.yRange:
        Object.keys(yaxis.limits.range).forEach(axis => {
          if (!uPlotInst.scales[axis]) return;
          uPlotInst.scales[axis].range = (u: uPlot, min = 0, max = 1) => getRangeY(u, min, max, axis);
        });
        break;
      case typeChartUpdate.data:
        uPlotInst.setData(data);
        break;
    }
    if (!isPanning) uPlotInst.redraw();
  };

  useEffect(() => setXRange({ min: period.start, max: period.end }), [period]);

  useEffect(() => {
    setStickyToolTips([]);
    setTooltipIdx({ seriesIdx: -1, dataIdx: -1 });
    if (!uPlotRef.current) return;
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    setXRange({ min: period.start, max: period.end });
    return u.destroy;
  }, [uPlotRef.current, series, layoutSize, height, isDarkTheme]);

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [xRange]);

  useEffect(() => updateChart(typeChartUpdate.data), [data]);
  useEffect(() => updateChart(typeChartUpdate.xRange), [xRange]);
  useEffect(() => updateChart(typeChartUpdate.yRange), [yaxis]);

  useEffect(() => {
    const show = tooltipIdx.dataIdx !== -1 && tooltipIdx.seriesIdx !== -1;
    setShowTooltip(show);

    if (show) window.addEventListener("click", handleClick);

    return () => {
      window.removeEventListener("click", handleClick);
    };
  }, [tooltipIdx, stickyTooltips]);

  return (
    <div
      className={classNames({
        "vm-line-chart": true,
        "vm-line-chart_panning": isPanning
      })}
    >
      <div
        className="vm-line-chart__u-plot"
        ref={uPlotRef}
      />
      {uPlotInst && showTooltip && (
        <ChartTooltip
          unit={unit}
          u={uPlotInst}
          series={series}
          metrics={metrics}
          yRange={yRange}
          tooltipIdx={tooltipIdx}
          tooltipOffset={tooltipOffset}
          id={tooltipId}
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
