import { beforeEach, describe, expect, it } from "vitest";
import { addRecentDuration, getRecentDurations, normalizeRecentDurations, saveRecentDuration } from "./utils";

describe("custom time range history", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("normalizes, deduplicates, and limits stored durations", () => {
    expect(normalizeRecentDurations(["1H", "bad", "1h", "2d", 3, "4w", "5m"]))
      .toEqual(["1h", "2d", "4w"]);
  });

  it("adds durations in most-recent-first order", () => {
    expect(addRecentDuration(["1h", "2h", "3h"], "2h"))
      .toEqual(["2h", "1h", "3h"]);
    expect(addRecentDuration(["2h", "1h", "3h"], "4h"))
      .toEqual(["4h", "2h", "1h"]);
  });

  it("persists recent durations", () => {
    saveRecentDuration("1h");
    saveRecentDuration("2d");
    saveRecentDuration("1h");
    expect(getRecentDurations()).toEqual(["1h", "2d"]);
  });
});
