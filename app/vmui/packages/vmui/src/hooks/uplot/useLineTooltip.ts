import uPlot, { Series as uPlotSeries } from "uplot";
import { useCallback, useEffect, useState } from "preact/compat";
import { ChartTooltipProps } from "../../components/Chart/ChartTooltip/ChartTooltip";
import { SeriesItem } from "../../types";
import get from "lodash.get";
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
  isAnomalyView?: boolean;
}

const useLineTooltip = ({ u, metrics, series, unit, isAnomalyView }: LineTooltipHook) => {
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

    const value = get(u, ["data", seriesIdx, dataIdx], 0);
    const min = get(u, ["scales", "1", "min"], 0);
    const max = get(u, ["scales", "1", "max"], 1);

    const date = get(u, ["data", 0, dataIdx], 0);

    const point = {
      top: u ? u.valToPos((value || 0), seriesItem?.scale || "1") : 0,
      left: u ? u.valToPos(date, "x") : 0,
    };

    return {
      unit,
      point,
      u: u,
      id: `${seriesIdx}_${dataIdx}`,
      title: groups.size > 1 && !isAnomalyView ? `Query ${group}` : "",
      dates: [date ? dayjs(date * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT) : "-"],
      value: formatPrettyNumber(value, min, max),
      info: getMetricName(metricItem),
      statsFormatted: seriesItem?.statsFormatted,
      marker: `${seriesItem?.stroke}`,
    };
  }, [u, tooltipIdx, metrics, series, unit, isAnomalyView]);

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
