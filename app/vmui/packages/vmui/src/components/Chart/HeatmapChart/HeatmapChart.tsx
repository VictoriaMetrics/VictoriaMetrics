import React, { FC, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
import uPlot, {
  AlignedData as uPlotData,
  Options as uPlotOptions,
  Range
} from "uplot";
import { defaultOptions, formatPrettyNumber, sizeAxis } from "../../../utils/uplot/helpers";
import { dragChart } from "../../../utils/uplot/events";
import { getAxes } from "../../../utils/uplot/axes";
import { MetricResult } from "../../../api/types";
import { dateFromSeconds, formatDateForNativeInput, limitsDurations } from "../../../utils/time";
import throttle from "lodash.throttle";
import useResize from "../../../hooks/useResize";
import { TimeParams } from "../../../types";
import { YaxisState } from "../../../state/graph/reducer";
import "uplot/dist/uPlot.min.css";
import classNames from "classnames";
import dayjs from "dayjs";
import { useAppState } from "../../../state/common/StateContext";
import { heatmapPaths } from "../../../utils/uplot/heatmap";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../../constants/date";
import ChartTooltipHeatmap, {
  ChartTooltipHeatmapProps,
  TooltipHeatmapProps
} from "../ChartTooltipHeatmap/ChartTooltipHeatmap";

export interface HeatmapChartProps {
  metrics: MetricResult[];
  data: uPlotData;
  period: TimeParams;
  yaxis: YaxisState;
  unit?: string;
  setPeriod: ({ from, to }: {from: Date, to: Date}) => void;
  container: HTMLDivElement | null;
  height?: number;
  onChangeLegend: (val: number) => void;
}

enum typeChartUpdate {xRange = "xRange", yRange = "yRange"}

const HeatmapChart: FC<HeatmapChartProps> = ({
  data,
  metrics = [],
  period,
  yaxis,
  unit,
  setPeriod,
  container,
  height,
  onChangeLegend,
}) => {
  const { isDarkTheme } = useAppState();

  const uPlotRef = useRef<HTMLDivElement>(null);
  const [isPanning, setPanning] = useState(false);
  const [xRange, setXRange] = useState({ min: period.start, max: period.end });
  const [uPlotInst, setUPlotInst] = useState<uPlot>();
  const [startTouchDistance, setStartTouchDistance] = useState(0);
  const layoutSize = useResize(container);

  const [tooltipProps, setTooltipProps] = useState<TooltipHeatmapProps | null>(null);
  const [tooltipOffset, setTooltipOffset] = useState({ left: 0, top: 0 });
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipHeatmapProps[]>([]);
  const tooltipId = useMemo(() => {
    return `${tooltipProps?.fields.join(",")}_${tooltipProps?.startDate}`;
  }, [tooltipProps]);

  const setScale = ({ min, max }: { min: number, max: number }): void => {
    if (isNaN(min) || isNaN(max)) return;
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
    if (!tooltipProps) return;
    const id = `${tooltipProps?.fields.join(",")}_${tooltipProps?.startDate}`;
    const props = {
      id,
      unit,
      tooltipOffset,
      ...tooltipProps
    };

    if (!stickyTooltips.find(t => t.id === id)) {
      const res = JSON.parse(JSON.stringify(props));
      setStickyToolTips(prev => [...prev, res]);
    }
  };

  const handleUnStick = (id:string) => {
    setStickyToolTips(prev => prev.filter(t => t.id !== id));
  };


  const setCursor = (u: uPlot) => {
    const left = u.cursor.left && u.cursor.left > 0 ? u.cursor.left : 0;
    const top = u.cursor.top && u.cursor.top > 0 ? u.cursor.top : 0;

    const xArr = (u.data[1][0] || []) as number[];
    if (!Array.isArray(xArr)) return;
    const xVal = u.posToVal(left, "x");
    const yVal = u.posToVal(top, "y");
    const xIdx = xArr.findIndex((t, i) => xVal >= t && xVal < xArr[i + 1]) || -1;
    const second = xArr[xIdx + 1];

    const result = metrics[Math.round(yVal)];
    if (!result) {
      setTooltipProps(null);
      return;
    }

    const metric = result?.metric;
    const metricName = metric["__name__"] || "value";

    const labelNames = Object.keys(metric).filter(x => x != "__name__");
    const fields = labelNames.map(key => `${key}=${JSON.stringify(metric[key])}`);

    const [endTime = 0, value = ""] = result.values.find(v => v[0] === second) || [];
    const valueFormat = formatPrettyNumber(+value, 0, 1);
    const startTime = xArr[xIdx];
    const startDate = dayjs(startTime * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT);
    const endDate = dayjs(endTime * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT);

    setTooltipProps({
      cursor: { left, top },
      startDate,
      endDate,
      metricName,
      fields,
      value: +value,
      valueFormat: valueFormat,
    });
  };

  const getRangeX = (): Range.MinMax => [xRange.min, xRange.max];

  const axes = getAxes( [{}], unit);
  const options: uPlotOptions = {
    ...defaultOptions,
    mode: 2,
    tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
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
    axes: [
      ...axes,
      {
        scale: "y",
        stroke: axes[0].stroke,
        font: axes[0].font,
        size: sizeAxis,
        splits: metrics.map((m, i) => i),
        values: metrics.map(m => Object.entries(m.metric).map(e => `${e[0]}=${JSON.stringify(e[1])}`)[0]),
      }
    ],
    scales: {
      x: {
        time: true,
      },
      y: {
        log: 2,
        time: false,
        range: (self, initMin, initMax) => [initMin - 1, initMax + 1]
      }
    },
    width: layoutSize.width || 400,
    height: height || 500,
    plugins: [{ hooks: { ready: onReadyChart, setCursor } }],
    hooks: {
      setSelect: [
        (u) => {
          const min = u.posToVal(u.select.left, "x");
          const max = u.posToVal(u.select.left + u.select.width, "x");
          setPlotScale({ u, min, max });
        }
      ]
    },
  };

  const updateChart = (type: typeChartUpdate): void => {
    if (!uPlotInst) return;
    switch (type) {
      case typeChartUpdate.xRange:
        uPlotInst.scales.x.range = getRangeX;
        break;
    }
    if (!isPanning) uPlotInst.redraw();
  };

  useEffect(() => setXRange({ min: period.start, max: period.end }), [period]);

  useEffect(() => {
    setStickyToolTips([]);
    setTooltipProps(null);
    if (!uPlotRef.current || !layoutSize.width || !layoutSize.height) return;
    const u = new uPlot(options, data, uPlotRef.current);
    setUPlotInst(u);
    setXRange({ min: period.start, max: period.end });
    return u.destroy;
  }, [uPlotRef.current, layoutSize, height, isDarkTheme, data]);

  useEffect(() => {
    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [xRange]);

  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 2) return;
    e.preventDefault();

    const dx = e.touches[0].clientX - e.touches[1].clientX;
    const dy = e.touches[0].clientY - e.touches[1].clientY;
    setStartTouchDistance(Math.sqrt(dx * dx + dy * dy));
  };

  const handleTouchMove = (e: TouchEvent) => {
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
      u: uPlotInst,
      min: min + zoomFactor,
      max: max - zoomFactor
    }));
  };

  useEffect(() => {
    window.addEventListener("touchmove", handleTouchMove);
    window.addEventListener("touchstart", handleTouchStart);

    return () => {
      window.removeEventListener("touchmove", handleTouchMove);
      window.removeEventListener("touchstart", handleTouchStart);
    };
  }, [uPlotInst, startTouchDistance]);

  useEffect(() => updateChart(typeChartUpdate.xRange), [xRange]);
  useEffect(() => updateChart(typeChartUpdate.yRange), [yaxis]);

  useEffect(() => {
    const show = !!tooltipProps?.value;
    if (show) window.addEventListener("click", handleClick);

    return () => {
      window.removeEventListener("click", handleClick);
    };
  }, [tooltipProps, stickyTooltips]);

  useEffect(() => {
    onChangeLegend(tooltipProps?.value || 0);
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
      {uPlotInst && tooltipProps && (
        <ChartTooltipHeatmap
          {...tooltipProps}
          unit={unit}
          u={uPlotInst}
          tooltipOffset={tooltipOffset}
          id={tooltipId}
        />
      )}

      {uPlotInst && stickyTooltips.map(t => (
        <ChartTooltipHeatmap
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

export default HeatmapChart;
