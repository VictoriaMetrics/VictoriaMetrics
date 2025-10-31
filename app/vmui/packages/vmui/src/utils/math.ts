import quickselect from "./quickselect";

export const roundToThousandths = (num: number): number => Math.round(num*1000)/1000;

type MathStatsOptions = {
  min?: boolean;
  max?: boolean;
  median?: boolean;
  avg?: boolean;
};

type MathStatsResult = {
  min: number | null;
  max: number | null;
  median: number | null;
  avg: number | null;
};

/**
 * Returns median of finite numbers in `vals`.
 * MUTATES `vals` in place (uses quickselect).
 */
const medianFromFiniteInPlace = (vals: number[]): number | null => {
  const m = vals.length;
  if (m === 0) return null;

  const k = m >> 1;
  quickselect(vals, k); // place upper median at vals[k]
  const upper = vals[k];

  if (m & 1) return upper; // odd length

  // even length: take max of the left half [0..k-1]
  let lower = -Infinity;
  for (let i = 0; i < k; i++) {
    const v = vals[i];
    if (v > lower) lower = v;
  }
  return (lower + upper) / 2;
};

export const getMathStats = (
  a: (number | null)[],
  ops: MathStatsOptions
): MathStatsResult => {
  const needMin = !!ops.min;
  const needMax = !!ops.max;
  const needAvg = !!ops.avg;
  const needMedian = !!ops.median;

  if (!needMin && !needMax && !needAvg && !needMedian) {
    return { min: null, max: null, median: null, avg: null };
  }

  // min & max
  let minVal = Infinity;
  let maxVal = -Infinity;

  // average
  let avgVal = 0;
  let avgCount = 0;

  // collect finite values for median
  const vals: number[] = [];

  for (const v of a) {
    if (v == null || !Number.isFinite(v)) continue;

    if (needMin && v < minVal) minVal = v;
    if (needMax && v > maxVal) maxVal = v;

    if (needAvg) {
      avgCount++;
      avgVal += (v - avgVal) / avgCount;
    }

    if (needMedian) vals.push(v);
  }

  return {
    min: Number.isFinite(minVal) ? minVal : null,
    max: Number.isFinite(maxVal) ? maxVal : null,
    avg: avgCount ? avgVal : null,
    median: (vals && needMedian) ? medianFromFiniteInPlace(vals) : null,
  };
};
