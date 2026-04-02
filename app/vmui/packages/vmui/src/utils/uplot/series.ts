import { MetricBase, MetricResult } from "../../api/types";
import uPlot, { Series as uPlotSeries } from "uplot";
import { getNameForMetric, promValueToNumber } from "../metric";
import { HideSeriesArgs, LegendItemType, SeriesItem } from "../../types";
import { baseContrastColors, getColorFromString } from "../color";
import { getMathStats } from "../math";
import { formatPrettyNumber } from "./helpers";
import { drawPoints } from "./scatter";

// Helper function to extract freeFormFields values as a comma-separated string
export const extractFields = (metric: MetricBase["metric"]): string => {
  const excludeMetrics = ["__name__", "for"];
  return Object.entries(metric)
    .filter(([key]) => !excludeMetrics.includes(key))
    .map(([key, value]) => `${key}: ${value}`).join(",");
};

export const getSeriesItemContext = (data: MetricResult[], hideSeries: string[], alias: string[], showPoints?: boolean, isRawQuery?: boolean) => {
  const colorState: {[key: string]: string} = {};
  const maxColors = Math.min(data.length, baseContrastColors.length);

  for (let i = 0; i < maxColors; i++) {
    const label = getNameForMetric(data[i], alias[data[i].group - 1]);
    colorState[label] = baseContrastColors[i];
  }

  return (d: MetricResult): SeriesItem => {
    const aliasValue = alias[d.group - 1];
    const label = getNameForMetric(d, aliasValue);

    return {
      label,
      hasAlias: Boolean(aliasValue),
      width: 1.4,
      stroke: colorState[label] || getColorFromString(label),
      points: getPointsSeries(showPoints, isRawQuery),
      spanGaps: false,
      freeFormFields: d.metric,
      show: !includesHideSeries(label, hideSeries),
      scale: "1",
      paths: isRawQuery ? drawPoints : undefined,
      ...getSeriesStatistics(d),
    };
  };
};

const getSeriesStatistics = (d: MetricResult) => {
  const values = d.values.map(v => promValueToNumber(v[1]));
  const { min, max, median } = getMathStats(values, { min: true, max: true, median: true });
  return {
    median: Number(median),
    statsFormatted: {
      min: formatPrettyNumber(min, min, max),
      max: formatPrettyNumber(max, min, max),
      median: formatPrettyNumber(median, min, max),
    },
  };
};

const getLabelForSeries = (s: uPlotSeries): string => typeof s.label === "string" ? s.label : "";

export const getLegendItem = (s: SeriesItem, group: number): LegendItemType => ({
  group,
  label: getLabelForSeries(s),
  color: s.stroke as string,
  checked: s.show || false,
  freeFormFields: s.freeFormFields,
  statsFormatted: s.statsFormatted,
  median: s.median,
  hasAlias: s.hasAlias || false,
});

export const getHideSeries = ({ hideSeries, legend, metaKey, series }: HideSeriesArgs): string[] => {
  const { label } = legend;
  const include = includesHideSeries(label, hideSeries);
  const labels = series.map(getLabelForSeries);

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

export const delSeries = (u: uPlot) => {
  for (let i = u.series.length - 1; i >= 0; i--) {
    i && u.delSeries(i);
  }
};

export const addSeries = (u: uPlot, series: uPlotSeries[], spanGaps = false, showPoints = false, isRawQuery?: boolean) => {
  series.forEach((s,i) => {
    if (s.label) s.spanGaps = spanGaps;
    if (s.points) s.points.filter = showPoints || isRawQuery ? undefined : filterPoints;
    i && u.addSeries(s);
  });
};

const getPointsSeries = (showPoints: boolean = false, isRawQuery?: boolean): uPlotSeries.Points => {
  return {
    size: isRawQuery ? 0 : 4,
    width: 0,
    show: true,
    filter: showPoints || isRawQuery ? null : filterPoints,
  };
};

const filterPoints = (self: uPlot, seriesIdx: number): number[] | null => {
  const data  = self.data[seriesIdx];
  const indices = [];

  for (let i = 0; i < data.length; i++) {
    const prev = data[i - 1];
    const next = data[i + 1];
    if (prev === null || next === null) {
      indices.push(i);
    }
  }

  return indices;
};
