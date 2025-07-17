import { describe, expect, it } from "vitest";
import { convertMetricsDataToCSV } from "./utils";
import { InstantMetricResult } from "../../api/types";

describe("convertMetricsDataToCSV", () => {
  it("should return an empty string if headers are empty", () => {
    const data: InstantMetricResult[] = [];
    expect(convertMetricsDataToCSV(data)).toBe("");
  });

  it("should return a valid CSV string for single metric entry with value", () => {
    const data: InstantMetricResult[] = [
      {
        value: [1623945600, "123"],
        group: 0,
        metric: {
          header1: "123",
          header2: "value2"
        }
      },
    ];
    const result = convertMetricsDataToCSV(data);
    expect(result).toBe("header1,header2\n123,value2");
  });

  it("should return a valid CSV string for multiple metric entries with values", () => {
    const data: InstantMetricResult[] = [
      {
        value: [1623945600, "123"],
        group: 0,
        metric: {
          header1: "123",
          header2: "value2"
        }
      },
      {
        value: [1623949200, "456"],
        group: 0,
        metric: {
          header1: "456",
          header2: "value4"
        }
      },
    ];
    const result = convertMetricsDataToCSV(data);
    expect(result).toBe("header1,header2\n123,value2\n456,value4");
  });

  it("should handle metric entries with multiple values field", () => {
    const data: InstantMetricResult[] = [
      { 
        values: [[1623945600, "123"], [1623949200, "456"]], 
        group: 0, 
        metric: {
          header1: "123-456",
          header2: "values"
        } 
      },
    ];
    const result = convertMetricsDataToCSV(data);
    expect(result).toBe("header1,header2\n123-456,values");
  });

  it("should handle a combination of metric entries with value and values", () => {
    const data: InstantMetricResult[] = [
      { 
        value: [1623945600, "123"], 
        group: 0, 
        metric: {
          header1: "123",
          header2: "first"
        } 
      },
      { 
        values: [[1623949200, "456"], [1623952800, "789"]], 
        group: 0, 
        metric: {
          header1: "456-789",
          header2: "second"
        } 
      },
    ];
    const result = convertMetricsDataToCSV(data);
    expect(result).toBe("header1,header2\n123,first\n456-789,second");
  });
});
