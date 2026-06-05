import uPlot, { Series as uPlotSeries } from "uplot";
import { useCallback, useEffect, useState } from "preact/compat";
import { ChartTooltipProps } from "../../components/Chart/ChartTooltip/ChartTooltip";
import { SeriesItem } from "../../types";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../constants/date";
import { getMetricName } from "../../utils/uplot";
import { MetricResult } from "../../api/types";
import useEventListener from "../useEventListener";

interface LineTooltipHook {
  u?: uPlot;
  metrics: MetricResult[];
  series: uPlotSeries[];
  unit?: string;
}

// Pixel proximity for detecting hover over null-timestamp X markers drawn at chart bottom.
const NULL_HOVER_PROX = 8;
// Half the visual marker height in CSS px (BASE_POINT_SIZE * 1.4 / 2 from scatter.ts).
// scatter.ts lifts the marker center by this amount above yMin so the icon sits inside
// the plot area; the hover y-anchor must match that offset.
const NULL_MARKER_HALF_CSS = 2.8;

interface NullHover {
  seriesIdx: number;
  timestamp: number;
}

const findNullHover = (u: uPlot): NullHover | null => {
  const cursorLeft = u.cursor.left ?? -1;
  const cursorTop = u.cursor.top ?? -1;
  if (cursorLeft < 0 || cursorTop < 0) return null;

  const scaleY = u.scales["1"];
  if (!scaleY || scaleY.min == null) return null;
  const yPos = u.valToPos(scaleY.min, "1") - NULL_MARKER_HALF_CSS;
  if (Math.abs(cursorTop - yPos) > NULL_HOVER_PROX) return null;

  let best: { seriesIdx: number; timestamp: number; dist: number } | null = null;
  for (let s = 1; s < u.series.length; s++) {
    const seriesItem = u.series[s] as SeriesItem;
    if (!seriesItem.show) continue;
    const nullTs = seriesItem.nullTimestamps;
    if (!nullTs || !nullTs.length) continue;

    for (let i = 0; i < nullTs.length; i++) {
      const t = nullTs[i];
      const xPos = u.valToPos(t, "x");
      const dist = Math.abs(cursorLeft - xPos);
      if (dist < NULL_HOVER_PROX && (best === null || dist < best.dist)) {
        best = { seriesIdx: s, timestamp: t, dist };
      }
    }
  }
  return best ? { seriesIdx: best.seriesIdx, timestamp: best.timestamp } : null;
};

const NULL_DESCRIPTION = "\"null\" can be a staleness marker or an actual NaN/null value produced by exporter.";

const useLineTooltip = ({ u, metrics, series, unit }: LineTooltipHook) => {
  const [showTooltip, setShowTooltip] = useState(false);
  const [tooltipIdx, setTooltipIdx] = useState({ seriesIdx: -1, dataIdx: -1 });
  const [nullTooltip, setNullTooltip] = useState<NullHover | null>(null);
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipProps[]>([]);

  const resetTooltips = () => {
    setStickyToolTips([]);
    setTooltipIdx({ seriesIdx: -1, dataIdx: -1 });
    setNullTooltip(null);
  };

  const setCursor = (u: uPlot) => {
    const dataIdx = u.cursor.idx ?? -1;
    setTooltipIdx(prev => ({ ...prev, dataIdx }));
    setNullTooltip(findNullHover(u));
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    const seriesIdx = sidx ?? -1;
    setTooltipIdx(prev => ({ ...prev, seriesIdx }));
  };

  const getNullTooltipProps = (hit: NullHover): ChartTooltipProps => {
    const { seriesIdx, timestamp } = hit;
    const metricItem = metrics[seriesIdx - 1];
    const seriesItem = series[seriesIdx] as SeriesItem;

    const groups = new Set(metrics.map(m => m.group));
    const group = metricItem?.group || 0;

    const yMin = u?.scales?.[1]?.min ?? 0;
    const point = {
      top: u ? u.valToPos(yMin, seriesItem?.scale || "1") - NULL_MARKER_HALF_CSS : 0,
      left: u ? u.valToPos(timestamp, "x") : 0,
    };

    return {
      u,
      id: `null_${seriesIdx}_${timestamp}`,
      title: groups.size > 1 ? `Query ${group}` : "",
      dates: [dayjs(timestamp * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT)],
      value: "null",
      info: getMetricName(metricItem, seriesItem),
      description: NULL_DESCRIPTION,
      marker: `${seriesItem?.stroke}`,
      point,
    };
  };

  const getTooltipProps = useCallback((): ChartTooltipProps => {
    if (nullTooltip) return getNullTooltipProps(nullTooltip);

    const { seriesIdx, dataIdx } = tooltipIdx;
    const metricItem = metrics[seriesIdx - 1];
    const seriesItem = series[seriesIdx] as SeriesItem;

    const groups = new Set(metrics.map(m => m.group));
    const group = metricItem?.group || 0;

    const value = u?.data?.[seriesIdx]?.[dataIdx] || 0;
    const date = u?.data?.[0]?.[dataIdx] || 0;

    let duplicateCount = 1;

    if (u && seriesIdx > 0 && dataIdx >= 0) {
      const xs = u.data[0] as (number | null)[];
      const ys = u.data[seriesIdx] as (number | null)[];

      const xVal = xs[dataIdx];
      const yVal = ys[dataIdx];

      if (xVal != null && yVal != null) {
        duplicateCount = 0;

        for (let i = 0; i < xs.length; i++) {
          if (xs[i] === xVal && ys[i] === yVal) {
            duplicateCount++;
          }
        }
      }
    }

    const point = {
      top: u ? u.valToPos((value || 0), seriesItem?.scale || "1") : 0,
      left: u ? u.valToPos(date, "x") : 0,
    };

    return {
      unit,
      point,
      u: u,
      id: `${seriesIdx}_${dataIdx}`,
      title: groups.size > 1 ? `Query ${group}` : "",
      dates: [date ? dayjs(date * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT) : "-"],
      value: value.toLocaleString("en-US", { maximumFractionDigits: 20 }),
      info: getMetricName(metricItem, seriesItem),
      statsFormatted: seriesItem?.statsFormatted,
      marker: `${seriesItem?.stroke}`,
      duplicateCount,
    };
  }, [u, tooltipIdx, metrics, series, unit, nullTooltip]);

  const handleClick = useCallback(() => {
    if (!showTooltip) return;
    const props = getTooltipProps();
    if (!stickyTooltips.find(t => t.id === props.id)) {
      setStickyToolTips(prev => [...prev, props]);
    }
  }, [getTooltipProps, stickyTooltips, showTooltip]);

  const handleUnStick = (id: string) => {
    setStickyToolTips(prev => prev.filter(t => t.id !== id));
  };

  useEffect(() => {
    const normalHit = tooltipIdx.dataIdx !== -1 && tooltipIdx.seriesIdx !== -1;
    setShowTooltip(normalHit || nullTooltip !== null);
  }, [tooltipIdx, nullTooltip]);

  useEventListener("click", handleClick);

  return {
    showTooltip,
    stickyTooltips,
    handleUnStick,
    getTooltipProps,
    seriesFocus,
    setCursor,
    resetTooltips,
  };
};

export default useLineTooltip;
