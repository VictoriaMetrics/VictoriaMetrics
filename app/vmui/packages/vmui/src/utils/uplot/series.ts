import { MetricBase, MetricResult } from "../../api/types";
import uPlot, { Series as uPlotSeries } from "uplot";
import { getNameForMetric, promValueToNumber } from "../metric";
import { BarSeriesItem, Disp, Fill, ForecastType, HideSeriesArgs, LegendItemType, SeriesItem, Stroke } from "../../types";
import { anomalyColors, baseContrastColors, getColorFromString } from "../color";
import { getLastFromArray, getMaxFromArray, getMedianFromArray, getMinFromArray } from "../math";
import { formatPrettyNumber } from "./helpers";

// Helper function to extract freeFormFields values as a comma-separated string
export const extractFields = (metric: MetricBase["metric"]): string => {
  const excludeMetrics = ["__name__", "for"];
  return Object.entries(metric)
    .filter(([key]) => !excludeMetrics.includes(key))
    .map(([key, value]) => `${key}: ${value}`).join(",");
};

type ForecastMetricInfo = {
  value: ForecastType | null;
  group: string;
}

export const isForecast = (metric: MetricBase["metric"]): ForecastMetricInfo => {
  const metricName = metric?.__name__ || "";
  const forecastRegex = new RegExp(`(${Object.values(ForecastType).join("|")})$`);
  const match = metricName.match(forecastRegex);
  const value = match && match[0] as ForecastType;
  const isY = /(?:^|[^a-zA-Z0-9_])y(?:$|[^a-zA-Z0-9_])/.test(metricName);
  return {
    value: isY ? ForecastType.actual : value,
    group: extractFields(metric)
  };
};

export const getSeriesItemContext = (data: MetricResult[], hideSeries: string[], alias: string[], isAnomalyUI?: boolean) => {
  const colorState: {[key: string]: string} = {};
  const maxColors = isAnomalyUI ? 0 : Math.min(data.length, baseContrastColors.length);

  for (let i = 0; i < maxColors; i++) {
    const label = getNameForMetric(data[i], alias[data[i].group - 1]);
    colorState[label] = baseContrastColors[i];
  }

  return (d: MetricResult, i: number): SeriesItem => {
    const metricInfo = isAnomalyUI ? isForecast(data[i].metric) : null;
    const label = isAnomalyUI ? metricInfo?.group || "" : getNameForMetric(d, alias[d.group - 1]);

    return {
      label,
      dash: getDashSeries(metricInfo),
      width: getWidthSeries(metricInfo),
      stroke: getStrokeSeries({ metricInfo, label, isAnomalyUI, colorState }),
      points: getPointsSeries(metricInfo),
      spanGaps: false,
      forecast: metricInfo?.value,
      forecastGroup: metricInfo?.group,
      freeFormFields: d.metric,
      show: !includesHideSeries(label, hideSeries),
      scale: "1",
      ...getSeriesStatistics(d),
    };
  };
};

const getSeriesStatistics = (d: MetricResult) => {
  const values = d.values.map(v => promValueToNumber(v[1]));
  const { min, max, median, last } = {
    min: getMinFromArray(values),
    max: getMaxFromArray(values),
    median: getMedianFromArray(values),
    last: getLastFromArray(values),
  };
  return {
    median,
    statsFormatted: {
      min: formatPrettyNumber(min, min, max),
      max: formatPrettyNumber(max, min, max),
      median: formatPrettyNumber(median, min, max),
      last: formatPrettyNumber(last, min, max),
    },
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

export const getHideSeries = ({ hideSeries, legend, metaKey, series, isAnomalyView }: HideSeriesArgs): string[] => {
  const { label } = legend;
  const include = includesHideSeries(label, hideSeries);
  const labels = series.map(s => s.label || "");

  // if anomalyView is true, always return all series except the one specified by `label`
  if (isAnomalyView) {
    return labels.filter(l => l !== label);
  }

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

export const addSeries = (u: uPlot, series: uPlotSeries[], spanGaps = false) => {
  series.forEach((s) => {
    if (s.label) s.spanGaps = spanGaps;
    u.addSeries(s);
  });
};

// Helpers

const getDashSeries = (metricInfo: ForecastMetricInfo | null): number[] => {
  const isLower = metricInfo?.value === ForecastType.yhatLower;
  const isUpper = metricInfo?.value === ForecastType.yhatUpper;
  const isYhat = metricInfo?.value === ForecastType.yhat;

  if (isLower || isUpper) {
    return [10, 5];
  } else if (isYhat) {
    return [10, 2];
  }
  return [];
};

const getWidthSeries = (metricInfo: ForecastMetricInfo | null): number => {
  const isLower = metricInfo?.value === ForecastType.yhatLower;
  const isUpper = metricInfo?.value === ForecastType.yhatUpper;
  const isYhat = metricInfo?.value === ForecastType.yhat;
  const isAnomalyMetric = metricInfo?.value === ForecastType.anomaly;

  if (isUpper || isLower) {
    return 0.7;
  } else if (isYhat) {
    return 1;
  } else if (isAnomalyMetric) {
    return 0;
  }
  return 1.4;
};

const getPointsSeries = (metricInfo: ForecastMetricInfo | null): uPlotSeries.Points => {
  const isAnomalyMetric = metricInfo?.value === ForecastType.anomaly;

  if (isAnomalyMetric) {
    return { size: 8, width: 4, space: 0 };
  }
  return { size: 4.2, width: 1.4 };
};

type GetStrokeSeriesArgs = {
  metricInfo: ForecastMetricInfo | null,
  label: string,
  colorState: {[p: string]: string},
  isAnomalyUI?: boolean
}

const getStrokeSeries = ({ metricInfo, label, isAnomalyUI, colorState }: GetStrokeSeriesArgs): uPlotSeries.Stroke => {
  const stroke: uPlotSeries.Stroke = colorState[label] || getColorFromString(label);
  const isAnomalyMetric = metricInfo?.value === ForecastType.anomaly;

  if (isAnomalyUI && isAnomalyMetric) {
    return anomalyColors[ForecastType.anomaly];
  } else if (isAnomalyUI && !isAnomalyMetric && !metricInfo?.value) {
    // TODO add stroke for training data
    // const hzGrad: [number, string][] = [
    //   [time, anomalyColors[ForecastType.actual]],
    //   [time, anomalyColors[ForecastType.training]],
    //   [time, anomalyColors[ForecastType.actual]],
    // ];
    // stroke = scaleGradient("x", 0, hzGrad, true);
    return anomalyColors[ForecastType.actual];
  } else if (metricInfo?.value) {
    return metricInfo?.value ? anomalyColors[metricInfo?.value] : stroke;
  }
  return colorState[label] || getColorFromString(label);
};
