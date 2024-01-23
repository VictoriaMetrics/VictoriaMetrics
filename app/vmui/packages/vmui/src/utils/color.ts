import { ArrayRGB, ForecastType } from "../types";

export const baseContrastColors = [
  "#e54040",
  "#32a9dc",
  "#2ee329",
  "#7126a1",
  "#e38f0f",
  "#3d811a",
  "#ffea00",
  "#2d2d2d",
  "#da42a6",
  "#a44e0c",
];

export const hexToRGB = (hex: string): string => {
  if (hex.length != 7) return "0, 0, 0";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `${r}, ${g}, ${b}`;
};

export const anomalyColors: Record<ForecastType, string> = {
  [ForecastType.yhatUpper]: "#7126a1",
  [ForecastType.yhatLower]: "#7126a1",
  [ForecastType.yhat]: "#da42a6",
  [ForecastType.anomaly]: "#da4242",
  [ForecastType.anomalyScore]: "#7126a1",
  [ForecastType.actual]: "#203ea9",
  [ForecastType.training]: `rgba(${hexToRGB("#203ea9")}, 0.2)`,
};

export const getColorFromString = (text: string): string => {
  const SEED = 16777215;
  const FACTOR = 49979693;

  let b = 1;
  let d = 0;
  let f = 1;

  if (text.length > 0) {
    for (let i = 0; i < text.length; i++) {
      text[i].charCodeAt(0) > d && (d = text[i].charCodeAt(0));
      f = parseInt(String(SEED / d));
      b = (b + text[i].charCodeAt(0) * f * FACTOR) % SEED;
    }
  }

  let hex = ((b * text.length) % SEED).toString(16);
  hex = hex.padEnd(6, hex);
  return `#${hex}`;
};

export const getContrastColor = (value: string) => {
  let hex = value.replace("#", "").trim();

  // convert 3-digit hex to 6-digits.
  if (hex.length === 3) {
    hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
  }

  if (hex.length !== 6) throw new Error("Invalid HEX color.");

  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  const yiq = ((r * 299) + (g * 587) + (b * 114)) / 1000;
  return yiq >= 128 ? "#000000" : "#FFFFFF";
};

export const generateGradient = (start: ArrayRGB, end: ArrayRGB, steps: number) => {
  const gradient = [];
  for (let i = 0; i < steps; i++) {
    const k = (i / (steps - 1));
    const r = start[0] + (end[0] - start[0]) * k;
    const g = start[1] + (end[1] - start[1]) * k;
    const b = start[2] + (end[2] - start[2]) * k;
    gradient.push([r, g, b].map(n => Math.round(n)).join(", "));
  }
  return gradient.map(c => `rgb(${c})`);
};
