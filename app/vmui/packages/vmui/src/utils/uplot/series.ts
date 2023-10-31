import { MetricResult } from "../../api/types";
import uPlot, { Series as uPlotSeries } from "uplot";
import { getNameForMetric, promValueToNumber } from "../metric";
import { HideSeriesArgs, BarSeriesItem, Disp, Fill, LegendItemType, Stroke, SeriesItem } from "../../types";
import { baseContrastColors, getColorFromString } from "../color";
import { getMedianFromArray, getMaxFromArray, getMinFromArray, getLastFromArray } from "../math";
import { formatPrettyNumber } from "./helpers";

export const getSeriesItemContext = (data: MetricResult[], hideSeries: string[], alias: string[]) => {
  const colorState: {[key: string]: string} = {};
  const stats = data.map(d => {
    const values = d.values.map(v => promValueToNumber(v[1]));
    return {
      min: getMinFromArray(values),
      max: getMaxFromArray(values),
      median: getMedianFromArray(values),
      last: getLastFromArray(values),
    };
  });

  const maxColors = Math.min(data.length, baseContrastColors.length);
  for (let i = 0; i < maxColors; i++) {
    const label = getNameForMetric(data[i], alias[data[i].group - 1]);
    colorState[label] = baseContrastColors[i];
  }

  return (d: MetricResult, i: number): SeriesItem => {
    const label = getNameForMetric(d, alias[d.group - 1]);
    const color = colorState[label] || getColorFromString(label);
    const { min, max, median, last } = stats[i];

    return {
      label,
      freeFormFields: d.metric,
      width: 1.4,
      stroke: color,
      show: !includesHideSeries(label, hideSeries),
      scale: "1",
      points: {
        size: 4.2,
        width: 1.4
      },
      statsFormatted: {
        min: formatPrettyNumber(min, min, max),
        max: formatPrettyNumber(max, min, max),
        median: formatPrettyNumber(median, min, max),
        last: formatPrettyNumber(last, min, max),
      },
      median: median,
    };
  };
};

export const getLegendItem = (s: SeriesItem, group: number): LegendItemType => ({
  group,
  label: s.label || "",
  color: s.stroke as string,
  checked: s.show || false,
  freeFormFields: s.freeFormFields,
  statsFormatted: s.statsFormatted,
  median: s.median,
});

export const getHideSeries = ({ hideSeries, legend, metaKey, series }: HideSeriesArgs): string[] => {
  const { label } = legend;
  const include = includesHideSeries(label, hideSeries);
  const labels = series.map(s => s.label || "");
  if (metaKey) {
    return include ? hideSeries.filter(l => l !== label) : [...hideSeries, label];
  } else if (hideSeries.length) {
    return include ? [...labels.filter(l => l !== label)] : [];
  } else {
    return [...labels.filter(l => l !== label)];
  }
};

export const includesHideSeries = (label: string, hideSeries: string[]): boolean => {
  return hideSeries.includes(`${label}`);
};

export const getBarSeries = (
  which: number[],
  ori: number,
  dir: number,
  radius: number,
  disp: Disp): BarSeriesItem => {
  return {
    which: which,
    ori: ori,
    dir: dir,
    radius: radius,
    disp: disp,
  };
};

export const barDisp = (stroke: Stroke, fill: Fill): Disp => {
  return {
    stroke: stroke,
    fill: fill
  };
};

export const delSeries = (u: uPlot) => {
  for (let i = u.series.length - 1; i >= 0; i--) {
    u.delSeries(i);
  }
};

export const addSeries = (u: uPlot, series: uPlotSeries[]) => {
  series.forEach((s) => {
    u.addSeries(s);
  });
};
