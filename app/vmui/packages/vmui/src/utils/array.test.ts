import { describe, expect, it } from "vitest";
import { isDecreasing } from "./array";

describe("isDecreasing", () => {
  it("should return true for an array with strictly decreasing numbers", () => {
    expect(isDecreasing([5, 4, 3, 2, 1])).toBe(true);
  });
 
  it("should return false for an array with increasing numbers", () => {
    expect(isDecreasing([1, 2, 3, 4, 5])).toBe(false);
  });

  it("should return false for an array with equal consecutive numbers", () => {
    expect(isDecreasing([5, 5, 4, 3, 2])).toBe(false);
  });

  it("should return false for an empty array", () => {
    expect(isDecreasing([])).toBe(false);
  });

  it("should return false for an array with a single element", () => {
    expect(isDecreasing([1])).toBe(false);
  });

  it("should return false for an array with both increasing and decreasing numbers", () => {
    expect(isDecreasing([5, 3, 4, 2, 1])).toBe(false);
  });

  it("should return true for an array with negative strictly decreasing numbers", () => {
    expect(isDecreasing([-1, -2, -3, -4])).toBe(true);
  });

  it("should return false for an array with a mix of positive and negative numbers that do not strictly decrease", () => {
    expect(isDecreasing([3, 2, -1, -1])).toBe(false);
  });
});
