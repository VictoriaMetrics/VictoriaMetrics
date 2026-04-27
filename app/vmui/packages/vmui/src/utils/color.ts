import { ArrayRGB } from "../types";

export const baseContrastColors = [
  "#e6194b", // red
  "#4363d8", // blue
  "#3cb44b", // green
  "#911eb4", // purple
  "#f58231", // orange
  "#f032e6", // magenta
  "#c8a200", // dark yellow
  "#a65628", // brown
  "#42d4f4", // cyan
  "#a9a9a9", // gray
];

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

const clamp = (n: number, min: number, max: number) => Math.min(max, Math.max(min, n));

const hexToRgb = (hex: string) => {
  let value = hex.replace("#", "").trim();

  if (value.length === 3) {
    value = value.split("").map((c) => c + c).join("");
  }

  if (!/^[0-9a-fA-F]{6}$/.test(value)) {
    throw new Error("Invalid HEX color.");
  }

  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16),
  };
};

const rgbToHex = (r: number, g: number, b: number) =>
  `#${[r, g, b].map((v) => clamp(Math.round(v), 0, 255).toString(16).padStart(2, "0")).join("")}`;

const rgbToHsl = (r: number, g: number, b: number) => {
  r /= 255; g /= 255; b /= 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  const d = max - min;

  let h = 0;
  let s = 0;

  if (d !== 0) {
    s = d / (1 - Math.abs(2 * l - 1));

    switch (max) {
      case r: h = ((g - b) / d) % 6; break;
      case g: h = (b - r) / d + 2; break;
      case b: h = (r - g) / d + 4; break;
    }

    h *= 60;
    if (h < 0) h += 360;
  }

  return { h, s: s * 100, l: l * 100 };
};

const hslToRgb = (h: number, s: number, l: number) => {
  s /= 100;
  l /= 100;

  const c = (1 - Math.abs(2 * l - 1)) * s;
  const x = c * (1 - Math.abs((h / 60) % 2 - 1));
  const m = l - c / 2;

  let r: number;
  let g: number;
  let b: number;

  if (h < 60) [r, g, b] = [c, x, 0];
  else if (h < 120) [r, g, b] = [x, c, 0];
  else if (h < 180) [r, g, b] = [0, c, x];
  else if (h < 240) [r, g, b] = [0, x, c];
  else if (h < 300) [r, g, b] = [x, 0, c];
  else [r, g, b] = [c, 0, x];

  return {
    r: (r + m) * 255,
    g: (g + m) * 255,
    b: (b + m) * 255,
  };
};

const varyColor = (hex: string, variant: number) => {
  const { r, g, b } = hexToRgb(hex);
  const { h, s, l } = rgbToHsl(r, g, b);

  const variants = [
    { ds: 0,   dl: 0   },
    { ds: -20, dl: -16 },
    { ds: -16, dl: +16 },
    { ds: +14, dl: -20  },
  ];

  const v = variants[variant % variants.length];

  const nextS = clamp(s + v.ds, 35, 85);
  const nextL = clamp(l + v.dl, 35, 70);

  const rgb = hslToRgb(h, nextS, nextL);
  return rgbToHex(rgb.r, rgb.g, rgb.b);
};

export const getSeriesColor = (index: number) => {
  const baseCount = baseContrastColors.length;

  const baseIndex = index % baseCount;
  const variantIndex = Math.floor(index / baseCount);

  const base = baseContrastColors[(baseIndex + variantIndex) % baseCount];

  return varyColor(base, variantIndex);
};
