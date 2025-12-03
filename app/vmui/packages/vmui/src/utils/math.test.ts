import { describe, it, expect } from "vitest";
import { getMathStats } from "./math";

const finite = (x: number) => Number.isFinite(x);

const expectedMin = (arr: number[]): number | null => {
  const vals = arr.filter(finite);
  return vals.length ? Math.min(...vals) : null;
};

const expectedMax = (arr: number[]): number | null => {
  const vals = arr.filter(finite);
  return vals.length ? Math.max(...vals) : null;
};

const expectedAvg = (arr: number[]): number | null => {
  const vals = arr.filter(finite);
  if (!vals.length) return null;
  const sum = vals.reduce((s, x) => s + x, 0);
  return sum / vals.length;
};

const expectedMedian = (arr: number[]): number | null => {
  const vals = arr.filter(finite).slice().sort((a, b) => a - b);
  const m = vals.length;
  if (!m) return null;
  const k = m >> 1;
  return m & 1 ? vals[k] : (vals[k - 1] + vals[k]) / 2;
};

describe("getMathStats — basics", () => {
  it("returns all nulls when no options are requested", () => {
    const a = [1, 2, 3];
    const r = getMathStats(a, {});
    expect(r).toEqual({ min: null, max: null, median: null, avg: null });
  });

  it("does not mutate the input array", () => {
    const a = [7, 3, 10, -2, 5];
    const before = a.slice();
    getMathStats(a, { min: true, max: true, median: true, avg: true });
    expect(a).toEqual(before);
  });
});

describe("getMathStats — individual metrics", () => {
  const arrays = [
    [7, 3, 10, -2, 5],
    [0, -0, 0, 0.5, 0.25, -1.25],
    [100],
    [NaN, Infinity, -Infinity, 42],
    [NaN, Infinity, -Infinity],
    [],
  ];

  it("min only", () => {
    for (const a of arrays) {
      const r = getMathStats(a, { min: true });
      expect(r.min).toBe(expectedMin(a));
      expect(r.max).toBeNull();
      expect(r.avg).toBeNull();
      expect(r.median).toBeNull();
    }
  });

  it("max only", () => {
    for (const a of arrays) {
      const r = getMathStats(a, { max: true });
      expect(r.max).toBe(expectedMax(a));
      expect(r.min).toBeNull();
      expect(r.avg).toBeNull();
      expect(r.median).toBeNull();
    }
  });

  it("average only", () => {
    for (const a of arrays) {
      const r = getMathStats(a, { avg: true });
      const exp = expectedAvg(a);
      if (exp == null) {
        expect(r.avg).toBeNull();
      } else {
        expect(r.avg!).toBeCloseTo(exp, 12);
      }
      expect(r.min).toBeNull();
      expect(r.max).toBeNull();
      expect(r.median).toBeNull();
    }
  });

  it("median only (odd/even, with non-finite filtered)", () => {
    const cases = [
      [7, 3, 10, -2, 5],              // odd
      [7, 3, 10, -2, 5, 8],           // even
      [NaN, Infinity, 3, 3, 3, 1],    // duplicates + non-finite
      [100],                          // single
      [],                             // empty
      [NaN, Infinity, -Infinity],     // only non-finite
    ];
    for (const a of cases) {
      const r = getMathStats(a, { median: true });
      const exp = expectedMedian(a);
      if (exp == null) {
        expect(r.median).toBeNull();
      } else {
        expect(r.median!).toBeCloseTo(exp, 12);
      }
      expect(r.min).toBeNull();
      expect(r.max).toBeNull();
      expect(r.avg).toBeNull();
    }
  });
});

describe("getMathStats — combined metrics", () => {
  it("all metrics together", () => {
    const a = [7, 3, 10, -2, 5, NaN, Infinity];
    const r = getMathStats(a, { min: true, max: true, avg: true, median: true });

    // expected values computed independently
    const expMin = expectedMin(a);
    const expMax = expectedMax(a);
    const expAvg = expectedAvg(a);
    const expMedian = expectedMedian(a);

    expect(r.min).toBe(expMin);
    expect(r.max).toBe(expMax);
    if (expAvg == null) expect(r.avg).toBeNull();
    else expect(r.avg!).toBeCloseTo(expAvg, 12);
    if (expMedian == null) expect(r.median).toBeNull();
    else expect(r.median!).toBeCloseTo(expMedian, 12);
  });

  it("does not return median when not requested", () => {
    const a = [9, 1, 5, 3, 7, 2, 11, 4];
    const r = getMathStats(a, { min: true, max: true, avg: true /* median: false */ });
    expect(r.min).toBe(expectedMin(a));
    expect(r.max).toBe(expectedMax(a));
    const expAvg = expectedAvg(a)!;
    expect(r.avg!).toBeCloseTo(expAvg, 12);
    // IMPORTANT: median should be null if not requested
    expect(r.median).toBeNull();
  });
});
