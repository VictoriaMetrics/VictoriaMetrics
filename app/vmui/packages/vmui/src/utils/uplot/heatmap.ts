import uPlot from "uplot";
import { generateGradient } from "../color";
import { MetricResult } from "../../api/types";
import { promValueToNumber } from "../metric";

// 16-color gradient from "rgb(246, 226, 219)" to "rgb(127, 39, 4)"
export const gradMetal16 = generateGradient([246, 226, 219], [127, 39, 4], 16);

export const countsToFills = (u: uPlot, seriesIdx: number) => {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  const counts = u.data[seriesIdx][2] as number[];
  const palette = gradMetal16;
  const hideThreshold = 0;

  let minCount = Infinity;
  let maxCount = -Infinity;

  for (let i = 0; i < counts.length; i++) {
    if (counts[i] > hideThreshold) {
      minCount = Math.min(minCount, counts[i]);
      maxCount = Math.max(maxCount, counts[i]);
    }
  }

  const range = maxCount - minCount;
  const paletteSize = palette.length;
  const indexedFills = Array(counts.length);

  for (let i = 0; i < counts.length; i++) {
    indexedFills[i] = counts[i] === 0
      ? -1
      : Math.min(paletteSize - 1, Math.floor((paletteSize * (counts[i] - minCount)) / range));
  }

  return indexedFills;
};

export const heatmapPaths = () => (u: uPlot, seriesIdx: number) => {
  const cellGap = Math.round(devicePixelRatio);

  uPlot.orient(u, seriesIdx, (
    series,
    dataX,
    dataY,
    scaleX,
    scaleY,
    valToPosX,
    valToPosY,
    xOff,
    yOff,
    xDim,
    yDim,
    moveTo,
    lineTo,
    rect
  ) => {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    const [xs, ys, counts] = u.data[seriesIdx] as [number[], number[], number[]];
    const dlen = xs.length;

    // fill colors are mapped from interpolating densities / counts along some gradient
    // (should be quantized to 64 colors/levels max. e.g. 16)
    const fills = countsToFills(u, seriesIdx);
    const fillPalette = gradMetal16 ?? [...Array.from(new Set(fills))];
    const fillPaths = fillPalette.map(() => new Path2D());

    // detect x and y bin qtys by detecting layout repetition in x & y data
    const yBinQty = dlen - ys.lastIndexOf(ys[0]);
    const xBinQty = dlen / yBinQty;
    const yBinIncr = ys[1] - ys[0];
    const xBinIncr = xs[yBinQty] - xs[0];

    // uniform tile sizes based on zoom level
    const xSize = (valToPosX(xBinIncr, scaleX, xDim, xOff) - valToPosX(0, scaleX, xDim, xOff)) - cellGap;
    const ySize = (valToPosY(yBinIncr, scaleY, yDim, yOff) - valToPosY(0, scaleY, yDim, yOff)) + cellGap;

    // pre-compute x and y offsets
    const cys = ys.slice(0, yBinQty).map((y: number) => {
      return Math.round(valToPosY(y, scaleY, yDim, yOff) - ySize / 2);
    });
    const cxs = Array.from({ length: xBinQty }, (v, i) => {
      return Math.round(valToPosX(xs[i * yBinQty], scaleX, xDim, xOff) - xSize);
    });

    for (let i = 0; i < dlen; i++) {
      // filter out 0 counts and out of view
      if (
        counts[i] > 0 &&
        xs[i] >= (scaleX.min || -Infinity) && xs[i] <= (scaleX.max || Infinity) &&
        ys[i] >= (scaleY.min || -Infinity) && ys[i] <= (scaleY.max || Infinity)
      ) {
        const cx = cxs[~~(i / yBinQty)];
        const cy = cys[i % yBinQty];

        const fillPath = fillPaths[fills[i]];

        rect(fillPath, cx, cy, xSize, ySize);
      }
    }

    u.ctx.save();
    u.ctx.rect(u.bbox.left, u.bbox.top, u.bbox.width, u.bbox.height);
    u.ctx.clip();
    fillPaths.forEach((p, i) => {
      u.ctx.fillStyle = fillPalette[i];
      u.ctx.fill(p);
    });
    u.ctx.restore();
  });
};

export const convertPrometheusToVictoriaMetrics = (buckets: MetricResult[]): MetricResult[] => {
  if (!buckets.every(a => a.metric.le)) return buckets;

  const sortedBuckets = buckets.sort((a,b) => parseFloat(a.metric.le) - parseFloat(b.metric.le));
  const group = buckets[0]?.group || 1;
  let prevBucket: MetricResult = { metric: { le: "" }, values: [], group };
  const result: MetricResult[] = [];

  for (const bucket of sortedBuckets) {
    const vmrange = [prevBucket.metric.le, bucket.metric.le].filter(n => n).join("...");
    const values: [number, string][] = [];

    for (const [timestamp, value] of bucket.values) {
      const prevVal = prevBucket.values.find(v => v[0] === timestamp)?.[1] || 0;
      const newVal = (+value) - (+prevVal);
      values.push([timestamp, `${newVal}`]);
    }

    result.push({ metric: { vmrange }, values, group });
    prevBucket = bucket;
  }

  return result;
};

const getUpperBound = (bucket: MetricResult) => {
  const values = (bucket.metric.vmrange || bucket.metric.le || "").split("...");
  return promValueToNumber(values[values.length - 1]);
};

const sortBucketsByValues = (a: MetricResult, b: MetricResult) => getUpperBound(a) - getUpperBound(b);

export const normalizeData = (buckets: MetricResult[], isHistogram?: boolean): MetricResult[] => {
  if (!isHistogram) return buckets;

  const sortedBuckets = buckets.sort(sortBucketsByValues);
  const vmBuckets = convertPrometheusToVictoriaMetrics(sortedBuckets);

  // Compute total hits for each timestamp upfront
  const totalHitsPerTimestamp: { [timestamp: number]: number } = {};
  vmBuckets.forEach(bucket =>
    bucket.values.forEach(([timestamp, value]) => {
      totalHitsPerTimestamp[timestamp] = (totalHitsPerTimestamp[timestamp] || 0) + +value;
    })
  );

  const result = vmBuckets.map(bucket => {
    const values = bucket.values.map(([timestamp, value]) => {
      const totalHits = totalHitsPerTimestamp[timestamp];
      return [timestamp, `${Math.round((+value / totalHits) * 100)}`];
    });

    return { ...bucket, values };
  }) as MetricResult[];

  return result.filter(r => !r.values.every(v => v[1] === "0"));
};
