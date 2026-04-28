import { describe, expect, it } from "vitest";
import { formatValueToCSV, getCSVExportColumns } from "./csv";

describe("formatValueToCSV", () => {
  it("should wrap value in quotes if it contains a comma", () => {
    const value = "hello,world";
    const result = formatValueToCSV(value);
    expect(result).toBe("\"hello,world\"");
  });

  it("should wrap value in quotes if it contains a newline", () => {
    const value = "hello\nworld";
    const result = formatValueToCSV(value);
    expect(result).toBe("\"hello\nworld\"");
  });

  it("should escape quotes and wrap in quotes if value contains a double quote", () => {
    const value = "hello \"world\"";
    const result = formatValueToCSV(value);
    expect(result).toBe("\"hello \"\"world\"\"\"");
  });

  it("should return the same value if it does not contain special characters", () => {
    const value = "hello world";
    const result = formatValueToCSV(value);
    expect(result).toBe("hello world");
  });

  it("should handle empty strings correctly", () => {
    const value = "";
    const result = formatValueToCSV(value);
    expect(result).toBe("");
  });
});

describe("getCSVExportColumns", () => {
  it("should prepend metric name and append value and timestamp columns", () => {
    const result = getCSVExportColumns(["instance", "__name__", "job", "instance"]);
    expect(result.join(",")).toEqual("__name__,instance,job,__value__,__timestamp__:unix_ms");
  });
});
