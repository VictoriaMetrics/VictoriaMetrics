import { describe, it, expect } from "vitest";
import quickselect from "./quickselect";

// Helper: verifies partition property around k using given comparator
function checkPartition<T>(
  arr: T[],
  k: number,
  compare: (a: T, b: T) => number
) {
  const pivot = arr[k];
  for (let i = 0; i < arr.length; i++) {
    if (i < k) {
      expect(compare(arr[i], pivot)).toBeLessThanOrEqual(0);
    } else if (i > k) {
      expect(compare(arr[i], pivot)).toBeGreaterThanOrEqual(0);
    }
  }
}

const numCmp = (a: number, b: number) => (a < b ? -1 : a > b ? 1 : 0);

describe("quickselect (numbers)", () => {
  it("finds k-th smallest on small array", () => {
    const arr = [7, 2, 9, 1, 5, 3];
    const k = 3;
    const orig = arr.slice();
    quickselect(arr, k);
    const sorted = orig.slice().sort(numCmp);
    expect(arr[k]).toBe(sorted[k]);
    checkPartition(arr, k, numCmp);
  });

  it("works for k = 0 (minimum)", () => {
    const arr = [10, -1, 4, 4, 2];
    const orig = arr.slice();
    quickselect(arr, 0);
    expect(arr[0]).toBe(Math.min(...orig));
    checkPartition(arr, 0, numCmp);
  });

  it("works for k = n-1 (maximum)", () => {
    const arr = [10, -1, 4, 4, 2];
    const k = arr.length - 1;
    const orig = arr.slice();
    quickselect(arr, k);
    expect(arr[k]).toBe(Math.max(...orig));
    checkPartition(arr, k, numCmp);
  });

  it("handles duplicates correctly", () => {
    const arr = [5, 1, 3, 3, 3, 2, 5, 4];
    const k = 4;
    const orig = arr.slice();
    quickselect(arr, k);
    const sorted = orig.slice().sort(numCmp);
    expect(arr[k]).toBe(sorted[k]);
    checkPartition(arr, k, numCmp);
  });

  it("handles negative numbers and mixed values", () => {
    const arr = [0, -100, 50, -3, 7, 7, 2, -1];
    const k = 2;
    const orig = arr.slice();
    quickselect(arr, k);
    const sorted = orig.slice().sort(numCmp);
    expect(arr[k]).toBe(sorted[k]);
    checkPartition(arr, k, numCmp);
  });

  it("matches fully sorted array at many random ks", () => {
    for (let t = 0; t < 25; t++) {
      const n = 50;
      const arr = Array.from({ length: n }, () => Math.floor(Math.random() * 1000) - 500);
      const k = Math.floor(Math.random() * n);
      const orig = arr.slice();
      quickselect(arr, k);
      const sorted = orig.slice().sort(numCmp);
      expect(arr[k]).toBe(sorted[k]);
      checkPartition(arr, k, numCmp);
    }
  });

  it("handles already sorted and reverse-sorted", () => {
    const asc = [1, 2, 3, 4, 5, 6, 7, 8, 9];
    const k1 = 4;
    const origAsc = asc.slice();
    quickselect(asc, k1);
    expect(asc[k1]).toBe(origAsc[k1]);
    checkPartition(asc, k1, numCmp);

    const desc = [9, 8, 7, 6, 5, 4, 3, 2, 1];
    const k2 = 3;
    const origDesc = desc.slice();
    quickselect(desc, k2);
    const sortedDesc = origDesc.slice().sort(numCmp);
    expect(desc[k2]).toBe(sortedDesc[k2]);
    checkPartition(desc, k2, numCmp);
  });

  it("handles all-equal values", () => {
    const arr = Array(100).fill(42);
    const k = 50;
    quickselect(arr, k);
    expect(arr[k]).toBe(42);
    checkPartition(arr, k, numCmp);
  });
});

describe("quickselect (custom comparator)", () => {
  it("works with strings (lexicographic)", () => {
    const arr = ["pear", "apple", "banana", "apricot", "kiwi"];
    const k = 2;
    const cmp = (a: string, b: string) => a.localeCompare(b);
    const orig = arr.slice();
    quickselect(arr, k, 0, arr.length - 1, cmp);
    const sorted = orig.slice().sort(cmp);
    expect(arr[k]).toBe(sorted[k]);
    checkPartition(arr, k, cmp);
  });

  it("works with objects via custom comparator (by score asc)", () => {
    type Item = { id: string; score: number };
    const items: Item[] = [
      { id: "a", score: 10 },
      { id: "b", score: -3 },
      { id: "c", score: 7 },
      { id: "d", score: 7 },
      { id: "e", score: 0 },
    ];
    const k = 3;
    const cmp = (x: Item, y: Item) => (x.score < y.score ? -1 : x.score > y.score ? 1 : 0);

    const orig = items.slice();
    quickselect(items, k, 0, items.length - 1, cmp);

    const sorted = orig.slice().sort(cmp);
    expect(items[k].score).toBe(sorted[k].score);
    checkPartition(items, k, cmp);
  });
});

describe("quickselect (bounds/segments)", () => {
  it("respects left/right boundaries (partial region)", () => {
    const arr = [9, 8, 7, 6, 5, 4, 3, 2, 1];
    // We'll only work inside [2..6], so k=4 is within that range.
    const left = 2;
    const right = 6;
    const k = 4;

    const before = arr.slice();
    quickselect(arr, k, left, right);

    // Elements outside [left..right] remain untouched
    expect(arr.slice(0, left)).toEqual(before.slice(0, left));
    expect(arr.slice(right + 1)).toEqual(before.slice(right + 1));

    // Inside the segment, property holds
    const seg = arr.slice(left, right + 1);
    const segCmp = numCmp;
    checkPartition(seg, k - left, segCmp);

    // And k-th inside the segment matches the k-th of the sorted segment
    const segSorted = before.slice(left, right + 1).sort(segCmp);
    expect(arr[k]).toBe(segSorted[k - left]);
  });

  it("single-element segment is a no-op", () => {
    const arr = [5, 4, 3, 2, 1];
    const left = 2, right = 2, k = 2;
    const before = arr.slice();
    quickselect(arr, k, left, right);
    expect(arr).toEqual(before);
  });

  it("k at segment boundaries (left/right)", () => {
    const arr1 = [7, 4, 6, 1, 9, 2, 5, 0];
    const left1 = 2, right1 = 6, k1 = left1;
    const segSorted1 = arr1.slice(left1, right1 + 1).slice().sort(numCmp);
    quickselect(arr1, k1, left1, right1);
    expect(arr1[k1]).toBe(segSorted1[0]);
    checkPartition(arr1.slice(left1, right1 + 1), k1 - left1, numCmp);

    const arr2 = [7, 4, 6, 1, 9, 2, 5, 0];
    const left2 = 1, right2 = 5, k2 = right2;
    const segSorted2 = arr2.slice(left2, right2 + 1).slice().sort(numCmp);
    quickselect(arr2, k2, left2, right2);
    expect(arr2[k2]).toBe(segSorted2[segSorted2.length - 1]);
    checkPartition(arr2.slice(left2, right2 + 1), k2 - left2, numCmp);
  });
});

describe("quickselect (Floyd–Rivest path)", () => {
  it("covers the large-array acceleration branch", () => {
    const n = 2000; // right - left = 1999 > 600 -> triggers acceleration
    const base = 2654435761; // Knuth multiplicative hash (32-bit after >>> 0)
    const arr = Array.from({ length: n }, (_, i) => ((i * base) >>> 0))
      .map(x => (x % 2000) - 1000); // keep numbers in a small range to include duplicates
    const k = Math.floor(n * 0.63);

    const orig = arr.slice();
    quickselect(arr, k);
    const sorted = orig.slice().sort(numCmp);

    expect(arr[k]).toBe(sorted[k]);
    checkPartition(arr, k, numCmp);
  });
});
