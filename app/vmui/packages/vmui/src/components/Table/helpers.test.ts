import { describe, it, expect } from "vitest";
import { descendingComparator } from "./helpers";
import { getNanoTimestamp } from "../../utils/time";

describe("descendingComparator", () => {
  it("returns 0 for equal numbers", () => {
    const result = descendingComparator({ value: 42 }, { value: 42 }, "value");
    expect(result).toBe(0);
  });

  it("sorts numbers descending", () => {
    const result = descendingComparator({ value: 100 }, { value: 50 }, "value");
    expect(result).toBeLessThan(0);
  });

  it("sorts null below any value", () => {
    expect(descendingComparator({ value: null }, { value: 10 }, "value")).toBe(1);
    expect(descendingComparator({ value: 10 }, { value: null }, "value")).toBe(-1);
    expect(descendingComparator({ value: null }, { value: null }, "value")).toBe(0);
  });

  it("sorts strings descending", () => {
    const result = descendingComparator({ name: "zzz" }, { name: "aaa" }, "name");
    expect(result).toBe(-1);
  });

  it("sorts numeric strings as numbers when possible", () => {
    const result = descendingComparator({ value: "200" }, { value: "50" }, "value");
    expect(result).toBeLessThan(0);
  });

  it("sorts date strings via getNanoTimestamp", () => {
    const a = { timestamp: "2024-01-01T00:00:00.200Z" };
    const b = { timestamp: "2023-01-01T00:00:00.100Z" };

    const nanoA = getNanoTimestamp(a.timestamp);
    const nanoB = getNanoTimestamp(b.timestamp);
    expect(nanoA).toBeGreaterThan(nanoB);

    const result = descendingComparator(a, b, "timestamp");
    expect(result).toBe(-1);
  });

  it("handles booleans and undefined safely", () => {
    expect(descendingComparator({ value: true }, { value: false }, "value")).toBe(-1);
    expect(descendingComparator({ value: undefined }, { value: false }, "value")).toBe(1);
  });
});
