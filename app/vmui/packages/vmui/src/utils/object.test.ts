import { describe, expect, it } from "vitest";
import { getValueByPath, isObject } from "./object";

describe("object", () => {
  describe("isObject", () => {
    it("should return true for a plain object", () => {
      const obj = { a: 1, b: 2 };
      expect(isObject(obj)).toBe(true);
    });

    it("should return true for a empty object", () => {
      const obj = { a: 1, b: 2 };
      expect(isObject(obj)).toBe(true);
    });

    it("should return false for null", () => {
      expect(isObject(null)).toBe(false);
    });

    it("should return false for an array", () => {
      expect(isObject([1, 2, 3])).toBe(false);
    });

    it("should return false for a string", () => {
      expect(isObject("string")).toBe(false);
    });

    it("should return false for a number", () => {
      expect(isObject(42)).toBe(false);
    });

    it("should return false for undefined", () => {
      expect(isObject(undefined)).toBe(false);
    });

    it("should return false for a function", () => {
      const fn = () => {};
      expect(isObject(fn)).toBe(false);
    });

    it("should return false for a boolean value", () => {
      expect(isObject(false)).toBe(false);
    });
  });

  describe("getValueByPath", () => {
    it("should return the value for a valid path", () => {
      const obj = { a: { b: { c: 42 } } };
      const result = getValueByPath(obj, "a.b.c");
      expect(result).toBe(42);
    });

    it("should return undefined for an invalid path", () => {
      const obj = { a: { b: { c: 42 } } };
      const result = getValueByPath(obj, "a.b.d");
      expect(result).toBeUndefined();
    });

    it("should return undefined if path includes non-object", () => {
      const obj = { a: { b: 42 } };
      const result = getValueByPath(obj, "a.b.c");
      expect(result).toBeUndefined();
    });

    it("should return the undefined object if path is empty", () => {
      const obj = { a: { b: { c: 42 } } };
      const result = getValueByPath(obj, "");
      expect(result).toBeUndefined();
    });

    it("should handle paths with single key", () => {
      const obj = { a: 42 };
      const result = getValueByPath(obj, "a");
      expect(result).toBe(42);
    });

    it("should handle arrays in the path", () => {
      const obj = { a: [{ b: 42 }] };
      const result = getValueByPath(obj, "a.0.b");
      expect(result).toBe(42);
    });

    it("should return undefined for a non-existent array index", () => {
      const obj = { a: [{ b: 42 }] };
      const result = getValueByPath(obj, "a.1.b");
      expect(result).toBeUndefined();
    });

    it("should return value for dot separated key", () => {
      const obj = { "foo.bar" : 1 };
      const result = getValueByPath(obj, "foo.bar");
      expect(result).toStrictEqual(1);
    });
  });
});
