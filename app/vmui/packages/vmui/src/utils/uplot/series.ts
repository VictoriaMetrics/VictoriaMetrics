import { MetricResult } from "../../api/types";
import { Series } from "uplot";
import { getNameForMetric } from "../metric";
import { BarSeriesItem, Disp, Fill, LegendItemType, Stroke } from "./types";
import { getColorLine } from "./helpers";
import { HideSeriesArgs } from "./types";

interface SeriesItem extends Series {
  freeFormFields: {[key: string]: string};
}

export const getSeriesItem = (d: MetricResult, hideSeries: string[], alias: string[]): SeriesItem => {
  const name = getNameForMetric(d, alias[d.group - 1]);
  const label = `[${d.group}]${name}`;
  return {
    label,
    freeFormFields: d.metric,
    width: 1.4,
    stroke: getColorLine(label),
    show: !includesHideSeries(label, hideSeries),
    scale: "1",
    points: {
      size: 4.2,
      width: 1.4
    }
  };
};

export const getLegendItem = (s: SeriesItem, group: number): LegendItemType => ({
  group,
  label: s.label || "",
  color: s.stroke as string,
  checked: s.show || false,
  freeFormFields: s.freeFormFields,
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
