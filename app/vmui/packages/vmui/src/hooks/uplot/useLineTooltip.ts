import uPlot, { Series as uPlotSeries } from "uplot";
import { useCallback, useEffect, useState } from "preact/compat";
import { ChartTooltipProps } from "../../components/Chart/ChartTooltip/ChartTooltip";
import { SeriesItem } from "../../types";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../constants/date";
import { formatPrettyNumber, getMetricName } from "../../utils/uplot";
import { MetricResult } from "../../api/types";
import useEventListener from "../useEventListener";

interface LineTooltipHook {
  u?: uPlot;
  metrics: MetricResult[];
  series: uPlotSeries[];
  unit?: string;
}

const useLineTooltip = ({ u, metrics, series, unit }: LineTooltipHook) => {
  const [showTooltip, setShowTooltip] = useState(false);
  const [tooltipIdx, setTooltipIdx] = useState({ seriesIdx: -1, dataIdx: -1 });
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipProps[]>([]);

  const resetTooltips = () => {
    setStickyToolTips([]);
    setTooltipIdx({ seriesIdx: -1, dataIdx: -1 });
  };

  const setCursor = (u: uPlot) => {
    const dataIdx = u.cursor.idx ?? -1;
    setTooltipIdx(prev => ({ ...prev, dataIdx }));
  };

  const seriesFocus = (u: uPlot, sidx: (number | null)) => {
    const seriesIdx = sidx ?? -1;
    setTooltipIdx(prev => ({ ...prev, seriesIdx }));
  };

  const getTooltipProps = useCallback((): ChartTooltipProps => {
    const { seriesIdx, dataIdx } = tooltipIdx;
    const metricItem = metrics[seriesIdx - 1];
    const seriesItem = series[seriesIdx] as SeriesItem;

    const groups = new Set(metrics.map(m => m.group));
    const group = metricItem?.group || 0;

    const value = u?.data?.[seriesIdx]?.[dataIdx] || 0;
    const min = u?.scales?.[1]?.min || 0;
    const max = u?.scales?.[1]?.max || 1;
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
      value: formatPrettyNumber(value, min, max),
      info: getMetricName(metricItem, seriesItem),
      statsFormatted: seriesItem?.statsFormatted,
      marker: `${seriesItem?.stroke}`,
      duplicateCount,
    };
  }, [u, tooltipIdx, metrics, series, unit]);

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
    setShowTooltip(tooltipIdx.dataIdx !== -1 && tooltipIdx.seriesIdx !== -1);
  }, [tooltipIdx]);

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
