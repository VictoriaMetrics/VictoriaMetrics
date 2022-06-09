import {MetricResult} from "../../api/types";
import {Series} from "uplot";
import {getNameForMetric} from "../metric";
import {BarSeriesItem, Disp, Fill, LegendItem, Stroke} from "./types";
import {getColorLine, getDashLine} from "./helpers";
import {HideSeriesArgs} from "./types";

interface SeriesItem extends Series {
  freeFormFields: {[key: string]: string};
}

export const getSeriesItem = (d: MetricResult, hideSeries: string[], alias: string[]): SeriesItem => {
  const label = getNameForMetric(d, alias[d.group - 1]);
  return {
    label,
    dash: getDashLine(d.group),
    freeFormFields: d.metric,
    width: 1.4,
    stroke: getColorLine(d.group, label),
    show: !includesHideSeries(label, d.group, hideSeries),
    scale: String(d.group),
    points: {
      size: 4.2,
      width: 1.4
    }
  };
};

export const getLegendItem = (s: SeriesItem, group: number): LegendItem => ({
  group,
  label: s.label || "",
  color: s.stroke as string,
  checked: s.show || false,
  freeFormFields: s.freeFormFields,
});

export const getHideSeries = ({hideSeries, legend, metaKey, series}: HideSeriesArgs): string[] => {
  const label = `${legend.group}.${legend.label}`;
  const include = includesHideSeries(legend.label, legend.group, hideSeries);
  const labels = series.map(s => `${s.scale}.${s.label}`);
  if (metaKey) {
    return include ? hideSeries.filter(l => l !== label) : [...hideSeries, label];
  } else if (hideSeries.length) {
    return include ? [...labels.filter(l => l !== label)] : [];
  } else {
    return [...labels.filter(l => l !== label)];
  }
};

export const includesHideSeries = (label: string, group: string | number, hideSeries: string[]): boolean => {
  return hideSeries.includes(`${group}.${label}`);
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
