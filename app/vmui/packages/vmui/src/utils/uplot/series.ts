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

export const isForecast = (metric: MetricBase["metric"]) => {
  const metricName = metric?.__name__ || "";
  const forecastRegex = new RegExp(`(${Object.values(ForecastType).join("|")})$`);
  const match = metricName.match(forecastRegex);
  const value = match && match[0] as ForecastType;
  return {
    value,
    isUpper: value === ForecastType.yhatUpper,
    isLower: value === ForecastType.yhatLower,
    isYhat: value === ForecastType.yhat,
    isAnomaly: value === ForecastType.anomaly,
    isAnomalyScore: value === ForecastType.anomalyScore,
    group: extractFields(metric)
  };
};

export const getSeriesItemContext = (data: MetricResult[], hideSeries: string[], alias: string[], isAnomaly?: boolean) => {
  const colorState: {[key: string]: string} = {};
  const maxColors = isAnomaly ? 0 : Math.min(data.length, baseContrastColors.length);

  for (let i = 0; i < maxColors; i++) {
    const label = getNameForMetric(data[i], alias[data[i].group - 1]);
    colorState[label] = baseContrastColors[i];
  }

  return (d: MetricResult, i: number): SeriesItem => {
    const forecast = isForecast(data[i].metric);
    const label = isAnomaly ? forecast.group : getNameForMetric(d, alias[d.group - 1]);

    const values = d.values.map(v => promValueToNumber(v[1]));
    const { min, max, median, last } = {
      min: getMinFromArray(values),
      max: getMaxFromArray(values),
      median: getMedianFromArray(values),
      last: getLastFromArray(values),
    };

    let dash: number[] = [];
    if (forecast.isLower || forecast.isUpper) {
      dash = [10, 5];
    } else if (forecast.isYhat) {
      dash = [10, 2];
    }

    let width = 1.4;
    if (forecast.isUpper || forecast.isLower) {
      width = 0.7;
    } else if (forecast.isYhat) {
      width = 1;
    } else if (forecast.isAnomaly) {
      width = 0;
    }

    let points: uPlotSeries.Points = { size: 4.2, width: 1.4 };
    if (forecast.isAnomaly) {
      points = { size: 8, width: 4, space: 0 };
    }

    let stroke: uPlotSeries.Stroke = colorState[label] || getColorFromString(label);
    if (isAnomaly && forecast.isAnomaly) {
      stroke = anomalyColors[ForecastType.anomaly];
    } else if (isAnomaly && !forecast.isAnomaly && !forecast.value) {
      // TODO add stroke for training data
      // const hzGrad: [number, string][] = [
      //   [time, anomalyColors[ForecastType.actual]],
      //   [time, anomalyColors[ForecastType.training]],
      //   [time, anomalyColors[ForecastType.actual]],
      // ];
      // stroke = scaleGradient("x", 0, hzGrad, true);
      stroke = anomalyColors[ForecastType.actual];
    } else if (forecast.value) {
      stroke = forecast.value ? anomalyColors[forecast.value] : stroke;
    }

    return {
      label,
      dash,
      width,
      stroke,
      points,
      spanGaps: false,
      forecast: forecast.value,
      forecastGroup: forecast.group,
      freeFormFields: d.metric,
      show: !includesHideSeries(label, hideSeries),
      scale: "1",
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

export const addSeries = (u: uPlot, series: uPlotSeries[], spanGaps = false) => {
  series.forEach((s) => {
    if (s.label) s.spanGaps = spanGaps;
    u.addSeries(s);
  });
};
