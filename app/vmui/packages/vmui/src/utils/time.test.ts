import { describe, it, expect } from "vitest";
import dayjs from "dayjs";
import {
  getCustomDurationFromRelativeTimeId,
  getCustomRelativeTimeId,
  getMillisecondsFromDuration,
  getNanoTimestamp,
  getRelativeTime,
  normalizeDuration,
  parseSupportedDuration
} from "./time";

describe("Time utils", () => {
  describe("getNanoTimestamp", () => {
    it("should return 0n for an empty string", () => {
      expect(getNanoTimestamp("")).toBe(0n);
    });

    it("correctly handles a date without a fractional part", () => {
      const dateStr = "2023-03-20T12:34:56Z";
      const baseMs = dayjs(dateStr).valueOf();
      const expected = BigInt(baseMs) * 1000000n;
      expect(getNanoTimestamp(dateStr)).toBe(expected);
    });

    it("correctly handles a date with a fractional part of 3 digits", () => {
      // In this case, the fractional part "123" is padded to "123000000",
      // and the remaining part after the first 3 digits is "000000"
      const dateStr = "2023-03-20T12:34:56.123Z";
      const baseMs = dayjs(dateStr).valueOf();
      const expected = BigInt(baseMs) * 1000000n; // extraNano = 0
      expect(getNanoTimestamp(dateStr)).toBe(expected);
    });

    it("correctly computes additional nanoseconds for a fractional part with more than 3 digits", () => {
      // For "123456", the fractional part is padded to "123456000",
      // extraNano = parseInt("456000") = 456000
      const dateStr = "2023-03-20T12:34:56.123456Z";
      const baseMs = dayjs(dateStr).valueOf();
      const extraNano = 456000n;
      const expected = BigInt(baseMs) * 1000000n + extraNano;
      expect(getNanoTimestamp(dateStr)).toBe(expected);
    });

    it("correctly handles a date with a fractional part of 9 digits", () => {
      // For "123456789", extraNano = parseInt("456789") = 456789
      const dateStr = "2023-03-20T12:34:56.123456789Z";
      const baseMs = dayjs(dateStr).valueOf();
      const extraNano = 456789n;
      const expected = BigInt(baseMs) * 1000000n + extraNano;
      expect(getNanoTimestamp(dateStr)).toBe(expected);
    });

    it("returns the default value if the regex does not match (e.g., missing \"Z\")", () => {
      const dateStr = "2023-03-20T12:34:56.123";
      const baseMs = dayjs(dateStr).valueOf();
      const expected = BigInt(baseMs) * 1000000n;
      expect(getNanoTimestamp(dateStr)).toBe(expected);
    });
  });

  describe("parseSupportedDuration (multi-unit)", () => {
    it("should parse single units", () => {
      expect(parseSupportedDuration("1h")).toEqual({ h: "1" });
      expect(parseSupportedDuration("1.5h")).toEqual({ h: "1.5" });
      expect(parseSupportedDuration(".5h")).toEqual({ h: ".5" });
    });

    it("should parse multiple units", () => {
      expect(parseSupportedDuration("1h30m")).toEqual({ h: "1", m: "30" });
      expect(parseSupportedDuration("1h 30m")).toEqual({ h: "1", m: "30" });
      expect(parseSupportedDuration("1h30.333m")).toEqual({ h: "1", m: "30.333" });
      expect(parseSupportedDuration("2h15s")).toEqual({ h: "2", s: "15" });
      expect(parseSupportedDuration("1.5h10m30s")).toEqual({ h: "1.5", m: "10", s: "30" });
    });

    it("should handle signs and commas", () => {
      expect(parseSupportedDuration("-2m")).toEqual({ m: "2" });
      expect(parseSupportedDuration("1,25h")).toEqual({ h: "1.25" });
    });

    it("should ignore spaces", () => {
      expect(parseSupportedDuration("  1h   30m")).toEqual({ h: "1", m: "30" });
    });

    it("should return undefined for unsupported or invalid input", () => {
      expect(parseSupportedDuration("1x")).toBeUndefined();
      expect(parseSupportedDuration("abc")).toBeUndefined();
      expect(parseSupportedDuration("")).toBeUndefined();
      expect(parseSupportedDuration("   ")).toBeUndefined();
    });
  });

  describe("getMillisecondsFromDuration", () => {
    it("should convert valid durations to milliseconds", () => {
      expect(getMillisecondsFromDuration("1s")).toBe(1000);
      expect(getMillisecondsFromDuration("1.5s")).toBe(1500);
      expect(getMillisecondsFromDuration("1m30s")).toBe(90000);
      expect(getMillisecondsFromDuration("1h 30m")).toBe(5400000);
      expect(getMillisecondsFromDuration("500ms")).toBe(500);
    });

    it("should return zero when duration cannot be parsed", () => {
      expect(getMillisecondsFromDuration("garbage")).toBe(0);
      expect(getMillisecondsFromDuration("")).toBe(0);
    });
  });

  describe("custom relative durations", () => {
    it("normalizes valid durations", () => {
      expect(normalizeDuration(" 1H 30m ")).toBe("1h30m");
      expect(normalizeDuration("1,5h")).toBe("1.5h");
      expect(normalizeDuration("500ms")).toBe("500ms");
    });

    it("rejects invalid durations", () => {
      expect(normalizeDuration("")).toBeUndefined();
      expect(normalizeDuration("0m")).toBeUndefined();
      expect(normalizeDuration("-2m")).toBeUndefined();
      expect(normalizeDuration("1m2h")).toBeUndefined();
      expect(normalizeDuration("1h2h")).toBeUndefined();
      expect(normalizeDuration("6y")).toBeUndefined();
      expect(normalizeDuration("1 hour")).toBeUndefined();
    });

    it("round-trips custom relative time ids", () => {
      const id = getCustomRelativeTimeId("4h");
      expect(id).toBe("last_4h");
      expect(getCustomDurationFromRelativeTimeId(id)).toBe("4h");
      expect(getCustomDurationFromRelativeTimeId("last_5_minutes")).toBeUndefined();
    });

    it("resolves a custom relative time against now", () => {
      const before = Date.now();
      const result = getRelativeTime({
        relativeTimeId: "last_4h",
        defaultDuration: "1h",
        defaultEndInput: new Date(0),
      });

      expect(result.relativeTimeId).toBe("last_4h");
      expect(result.duration).toBe("4h");
      expect(result.endInput.valueOf()).toBeGreaterThanOrEqual(before);
      expect(result.endInput.valueOf()).toBeLessThanOrEqual(Date.now());
    });
  });
});
