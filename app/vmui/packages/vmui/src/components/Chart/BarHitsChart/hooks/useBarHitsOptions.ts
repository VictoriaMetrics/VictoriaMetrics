import { useMemo, useState } from "preact/compat";
import { getAxes, getMinMaxBuffer, handleDestroy, setSelect } from "../../../../utils/uplot";
import dayjs from "dayjs";
import { dateFromSeconds, formatDateForNativeInput } from "../../../../utils/time";
import uPlot, { AlignedData, Band, Options, Series } from "uplot";
import { getCssVariable } from "../../../../utils/theme";
import { useAppState } from "../../../../state/common/StateContext";
import { MinMax, SetMinMax } from "../../../../types";
import { LogHits } from "../../../../api/types";
import getSeriesPaths from "../../../../utils/uplot/paths";
import { GraphOptions, GRAPH_STYLES } from "../types";
import { getMaxFromArray } from "../../../../utils/math";

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

export const OTHER_HITS_LABEL = "other";

export const getLabelFromLogHit = (logHit: LogHits) => {
  if (logHit?._isOther) return OTHER_HITS_LABEL;
  const fields = Object.values(logHit?.fields || {});
  return fields.map((value) => value || "\"\"").join(", ");
};

const getYRange = (u: uPlot, _initMin = 0, initMax = 1) => {
  const maxValues = u.series.filter(({ scale }) => scale === "y").map(({ max }) => max || initMax);
  const max = getMaxFromArray(maxValues);
  return getMinMaxBuffer(0, max || initMax);
};

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
    let visibleColorIndex = 0;

    return data.map((_d, i) => {
      if (i === 0) return {}; // x-axis

      const logHit = logHits?.[i - 1];
      const label = getLabelFromLogHit(logHit);

      const isOther = logHit?._isOther;
      const colorVar = isOther
        ? "color-log-hits-bar-0"
        : seriesColors[visibleColorIndex++];

      const color = getCssVariable(colorVar);

      return {
        label,
        width: strokeWidth[graphOptions.graphStyle],
        spanGaps: true,
        show: true,
        stroke: color,
        fill: graphOptions.fill && !isOther ? `${color}80` : graphOptions.fill ? color : "",
        paths: getSeriesPaths(graphOptions.graphStyle),
      };
    });
  }, [isDarkTheme, data, graphOptions]);

  const options: Options = {
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
      },
      y: {
        range: getYRange
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
  };

  return {
    options,
    series,
    focusDataIdx,
  };
};

export default useBarHitsOptions;
