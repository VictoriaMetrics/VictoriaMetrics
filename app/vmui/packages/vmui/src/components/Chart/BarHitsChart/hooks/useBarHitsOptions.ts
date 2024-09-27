import { useMemo, useState } from "preact/compat";
import { getAxes, handleDestroy, setSelect } from "../../../../utils/uplot";
import dayjs from "dayjs";
import { dateFromSeconds, formatDateForNativeInput } from "../../../../utils/time";
import uPlot, { AlignedData, Band, Options, Series } from "uplot";
import { getCssVariable } from "../../../../utils/theme";
import { useAppState } from "../../../../state/common/StateContext";
import { MinMax, SetMinMax } from "../../../../types";
import { LogHits } from "../../../../api/types";
import getSeriesPaths from "../../../../utils/uplot/paths";
import { GraphOptions, GRAPH_STYLES } from "../types";

const seriesColors = [
  "color-log-hits-bar-1",
  "color-log-hits-bar-2",
  "color-log-hits-bar-3",
  "color-log-hits-bar-4",
  "color-log-hits-bar-5",
];

const strokeWidth = {
  [GRAPH_STYLES.BAR]: 1,
  [GRAPH_STYLES.LINE_STEPPED]: 2,
  [GRAPH_STYLES.LINE]: 1.2,
  [GRAPH_STYLES.POINTS]: 0,
};

interface UseGetBarHitsOptionsArgs {
  data: AlignedData;
  logHits: LogHits[];
  xRange: MinMax;
  bands?: Band[];
  containerSize: { width: number, height: number };
  setPlotScale: SetMinMax;
  onReadyChart: (u: uPlot) => void;
  graphOptions: GraphOptions;
}

const useBarHitsOptions = ({
  data,
  logHits,
  xRange,
  bands,
  containerSize,
  onReadyChart,
  setPlotScale,
  graphOptions
}: UseGetBarHitsOptionsArgs) => {
  const { isDarkTheme } = useAppState();

  const [focusDataIdx, setFocusDataIdx] = useState(-1);

  const setCursor = (u: uPlot) => {
    const dataIdx = u.cursor.idx ?? -1;
    setFocusDataIdx(dataIdx);
  };

  const series: Series[] = useMemo(() => {
    let colorN = 0;
    return data.map((_d, i) => {
      if (i === 0) return {}; // 0 index is xAxis(timestamps)
      const fields = Object.values(logHits?.[i - 1]?.fields || {});
      const label = fields.map((value) => value || "\"\"").join(", ");
      const color = getCssVariable(label ? seriesColors[colorN] : "color-log-hits-bar-0");
      if (label) colorN++;
      return {
        label: label || "other",
        width: strokeWidth[graphOptions.graphStyle],
        spanGaps: true,
        stroke: color,
        fill: graphOptions.fill ? color + "80" : "",
        paths: getSeriesPaths(graphOptions.graphStyle),
      };
    });
  }, [isDarkTheme, data, graphOptions]);

  const options: Options = useMemo(() => ({
    series,
    bands,
    width: containerSize.width || (window.innerWidth / 2),
    height: containerSize.height || 200,
    cursor: {
      points: {
        width: (u, seriesIdx, size) => size / 4,
        size: (u, seriesIdx) => (u.series?.[seriesIdx]?.points?.size || 1) * 1.5,
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
      drawSeries: [],
      ready: [onReadyChart],
      setCursor: [setCursor],
      setSelect: [setSelect(setPlotScale)],
      destroy: [handleDestroy],
    },
    legend: { show: false },
    axes: getAxes([{}, { scale: "y" }]),
    tzDate: ts => dayjs(formatDateForNativeInput(dateFromSeconds(ts))).local().toDate(),
  }), [isDarkTheme, series, bands]);

  return {
    options,
    series,
    focusDataIdx,
  };
};

export default useBarHitsOptions;
