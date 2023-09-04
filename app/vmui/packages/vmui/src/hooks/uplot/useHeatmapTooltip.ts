import uPlot from "uplot";
import { useCallback, useState } from "preact/compat";
import { ChartTooltipProps } from "../../components/Chart/ChartTooltip/ChartTooltip";
import dayjs from "dayjs";
import { DATE_FULL_TIMEZONE_FORMAT } from "../../constants/date";
import { MetricResult } from "../../api/types";
import useEventListener from "../useEventListener";
import get from "lodash.get";

interface LineTooltipHook {
  u?: uPlot;
  metrics: MetricResult[];
  unit?: string;
}

const useLineTooltip = ({ u, metrics, unit }: LineTooltipHook) => {
  const [point, setPoint] = useState({ left: 0, top: 0 });
  const [stickyTooltips, setStickyToolTips] = useState<ChartTooltipProps[]>([]);

  const resetTooltips = () => {
    setStickyToolTips([]);
    setPoint({ left: 0, top: 0 });
  };

  const setCursor = (u: uPlot) => {
    const left = u.cursor.left || 0;
    const top = u.cursor.top || 0;
    setPoint({ left, top });
  };

  const getTooltipProps = useCallback((): ChartTooltipProps => {
    const { left, top } = point;
    const xArr = (get(u, ["data", 1, 0], []) || []) as number[];
    const xVal = u ? u.posToVal(left, "x") : 0;
    const yVal = u ? u.posToVal(top, "y") : 0;
    const xIdx = xArr.findIndex((t, i) => xVal >= t && xVal < xArr[i + 1]) || -1;
    const second = xArr[xIdx + 1];
    const result = metrics[Math.round(yVal)] || { values: [] };

    const [endTime = 0, value = ""] = result.values.find(v => v[0] === second) || [];
    const startTime = xArr[xIdx];
    const startDate = dayjs(startTime * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT);
    const endDate = dayjs(endTime * 1000).tz().format(DATE_FULL_TIMEZONE_FORMAT);

    const bucket = result?.metric?.vmrange || "";

    return {
      unit,
      point,
      u: u,
      id: `${bucket}_${startDate}`,
      dates: [startDate, endDate],
      value: `${value}%`,
      info: bucket,
      show: +value > 0
    };
  }, [u, point, metrics, unit]);

  const handleClick = useCallback(() => {
    const props = getTooltipProps();
    if (!props.show) return;
    if (!stickyTooltips.find(t => t.id === props.id)) {
      setStickyToolTips(prev => [...prev, props]);
    }
  }, [getTooltipProps, stickyTooltips]);

  const handleUnStick = (id: string) => {
    setStickyToolTips(prev => prev.filter(t => t.id !== id));
  };

  useEventListener("click", handleClick);

  return {
    stickyTooltips,
    handleUnStick,
    getTooltipProps,
    setCursor,
    resetTooltips
  };
};

export default useLineTooltip;
