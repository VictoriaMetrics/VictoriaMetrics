import { MinMax } from "../../types";
import { limitsDurations } from "../../utils/time";
import { useEffect, useState } from "preact/compat";
import { TimeParams } from "../../types";
import dayjs from "dayjs";

interface PlotScaleHook {
  setPeriod: ({ from, to }: { from: Date, to: Date }) => void;
  period: TimeParams;
}

const usePlotScale = ({ period, setPeriod }: PlotScaleHook) => {
  const [xRange, setXRange] = useState({ min: period.start, max: period.end });

  const setPlotScale = ({ min, max }: MinMax) => {
    const delta = (max - min) * 1000;
    if ((delta < limitsDurations.min) || (delta > limitsDurations.max)) return;
    setPeriod({
      from: dayjs(min * 1000).toDate(),
      to: dayjs(max * 1000).toDate()
    });
  };

  useEffect(() => {
    setXRange({ min: period.start, max: period.end });
  }, [period]);

  return {
    xRange,
    setPlotScale
  };
};

export default usePlotScale;
