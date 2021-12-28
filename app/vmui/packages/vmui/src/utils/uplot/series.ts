import {MetricResult} from "../../api/types";
import {Series} from "uplot";
import {getNameForMetric} from "../metric";
import {LegendItem} from "./types";
import {getColorLine, getDashLine} from "./helpers";
import {HideSeriesArgs} from "./types";

export const getSeriesItem = (d: MetricResult, hideSeries: string[]): Series => {
  const label = getNameForMetric(d);
  return {
    label,
    dash: getDashLine(d.group),
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

export const getLegendItem = (s: Series, group: number): LegendItem => ({
  group,
  label: s.label || "",
  color: s.stroke as string,
  checked: s.show || false
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