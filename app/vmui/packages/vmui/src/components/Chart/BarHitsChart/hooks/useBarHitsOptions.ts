import { useMemo, useState } from "preact/compat";
import { getAxes, handleDestroy, setSelect } from "../../../../utils/uplot";
import dayjs from "dayjs";
import { dateFromSeconds, formatDateForNativeInput } from "../../../../utils/time";
import uPlot, { Options } from "uplot";
import { getCssVariable } from "../../../../utils/theme";
import { barPaths } from "../../../../utils/uplot/bars";
import { useAppState } from "../../../../state/common/StateContext";
import { MinMax, SetMinMax } from "../../../../types";

interface UseGetBarHitsOptionsArgs {
  xRange: MinMax;
  containerSize: { width: number, height: number };
  setPlotScale: SetMinMax;
  onReadyChart: (u: uPlot) => void;
}

const useBarHitsOptions = ({ xRange, containerSize, onReadyChart, setPlotScale }: UseGetBarHitsOptionsArgs) => {
  const { isDarkTheme } = useAppState();

  const [focusDataIdx, setFocusDataIdx] = useState(-1);

  const series = useMemo(() => [
    {},
    {
      label: "y",
      width: 1,
      stroke: getCssVariable("color-log-hits-bar"),
      fill: getCssVariable("color-log-hits-bar"),
      paths: barPaths,
    }
  ], [isDarkTheme]);

  const setCursor = (u: uPlot) => {
    const dataIdx = u.cursor.idx ?? -1;
    setFocusDataIdx(dataIdx);
  };

  const options: Options = useMemo(() => ({
    series,
    width: containerSize.width || (window.innerWidth / 2),
    height: containerSize.height || 200,
    cursor: {
      points: {
        width: (u, seriesIdx, size) => size / 4,
        size: (u, seriesIdx) => (u.series?.[seriesIdx]?.points?.size || 1) * 2.5,
        stroke: (u, seriesIdx) => `${series?.[seriesIdx]?.stroke || "#ffffff"}`,
        fill: () => "#ffffff",
      },
    },
    scales: {
      x: {
        time: true,
        range: () => [xRange.min, xRange.max]
      }
    },
    hooks: {
      ready: [onReadyChart],
      setCursor: [setCursor],
      setSelect: [setSelect(setPlotScale)],
      destroy: [handleDestroy],
    },
    legend: { show: false },
    axes: getAxes([{}, { scale: "y" }]),
    tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
  }), [isDarkTheme]);

  return {
    options,
    focusDataIdx,
  };
};

export default useBarHitsOptions;
