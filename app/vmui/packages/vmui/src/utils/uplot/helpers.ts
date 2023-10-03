import uPlot from "uplot";
import { MetricResult } from "../../api/types";

export const formatTicks = (u: uPlot, ticks: number[], unit = ""): string[] => {
  const min = ticks[0];
  const max = ticks[ticks.length-1];
  if (!unit) {
    return ticks.map(v => formatPrettyNumber(v, min, max));
  }
  return ticks.map(v => `${formatPrettyNumber(v, min, max)} ${unit}`);
};

export const formatPrettyNumber = (
  n: number | null | undefined,
  min: number | null | undefined,
  max: number | null | undefined
): string => {
  if (n === undefined || n === null) {
    return "";
  }
  max = max || 0;
  min = min || 0;
  const range = Math.abs(max - min);
  if (isNaN(range) || range == 0) {
    // Return the constant number as is if the range isn't set of it is too small.
    if (Math.abs(n) >= 1000) {
      return n.toLocaleString("en-US");
    }
    return n.toString();
  }
  // Make sure n has 3 significant digits on the given range.
  // This precision should be enough for most UX cases,
  // since the remaining digits are usually a white noise.
  let digits = 3 + Math.floor(1 + Math.log10(Math.max(Math.abs(min), Math.abs(max))) - Math.log10(range));
  if (isNaN(digits) || digits > 20) digits = 20;
  return n.toLocaleString("en-US", {
    minimumSignificantDigits: 1,
    maximumSignificantDigits: digits,
  });
};

export const getTextWidth = (val: string, font: string): number => {
  const span = document.createElement("span");
  span.innerText = val;
  span.style.cssText = `position: absolute; z-index: -1; pointer-events: none; opacity: 0; font: ${font}`;
  document.body.appendChild(span);
  const width = span.offsetWidth;
  span.remove();
  return width;
};

export const getDashLine = (group: number): number[] => {
  return group <= 1 ? [] : [group*4, group*1.2];
};

export const getMetricName = (metricItem: MetricResult) => {
  const metric = metricItem?.metric || {};
  const labelNames = Object.keys(metric).filter(x => x != "__name__");
  const labels = labelNames.map(key => `${key}=${JSON.stringify(metric[key])}`);
  let metricName = metric["__name__"] || "";
  if (labels.length > 0) {
    metricName += "{" + labels.join(",") + "}";
  }
  return metricName;
};
