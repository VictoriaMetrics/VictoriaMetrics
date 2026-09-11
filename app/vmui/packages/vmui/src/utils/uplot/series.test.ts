import { describe, expect, it } from "vitest";
import { MetricResult } from "../../api/types";
import { getSeriesItemContext } from "./series";

const getStats = (values: string[]) => {
  const metric: MetricResult = {
    group: 1,
    metric: { __name__: "test_metric" },
    values: values.map((value, index) => [index, value]),
  };

  return getSeriesItemContext([metric], [], [""])(metric).statsFormatted;
};

describe("getSeriesItemContext", () => {
  it("calculates the range from the minimum and maximum values", () => {
    expect(getStats(["1.84", "2.3"]).range).toBe("0.46");
  });

  it("calculates the range for the values from the reported example", () => {
    expect(getStats(["12.88", "13.5", "14.07", "1855", "16.08"])).toEqual({
      min: "12.88",
      max: "1,855",
      median: "14.07",
      range: "1,842",
      last: "16.08",
    });
  });

  it("returns a zero range for a constant series", () => {
    expect(getStats(["5", "5"]).range).toBe("0");
  });

  it("ignores non-finite values and leaves the range empty without finite values", () => {
    expect(getStats(["NaN", "-Inf", "-5", "3", "+Inf"]).range).toBe("8");
    expect(getStats(["NaN", "-Inf", "+Inf"]).range).toBe("");
  });
});
