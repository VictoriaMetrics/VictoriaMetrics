import { describe, expect, it } from "vitest";
import { formatBytes } from "./bytes";

describe("formatBytes", () => {
  it("returns null for invalid values", () => {
    expect(formatBytes(-1)).toBeNull();
    expect(formatBytes(Number.NaN)).toBeNull();
    expect(formatBytes(Number.POSITIVE_INFINITY)).toBeNull();
    expect(formatBytes(Number.NEGATIVE_INFINITY)).toBeNull();
  });

  it("formats zero bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("formats bytes", () => {
    expect(formatBytes(0.5)).toBe("0.5 B");
    expect(formatBytes(1)).toBe("1 B");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });

  it("formats kilobytes", () => {
    expect(formatBytes(1024)).toBe("1 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
  });

  it("formats megabytes", () => {
    expect(formatBytes(1024 ** 2)).toBe("1 MB");
    expect(formatBytes(2.5 * 1024 ** 2)).toBe("2.5 MB");
  });

  it("formats gigabytes, terabytes and petabytes", () => {
    expect(formatBytes(1024 ** 3)).toBe("1 GB");
    expect(formatBytes(1024 ** 4)).toBe("1 TB");
    expect(formatBytes(1024 ** 5)).toBe("1 PB");
  });

  it("caps values above PB to PB unit", () => {
    expect(formatBytes(1024 ** 6)).toBe("1024 PB");
  });

  it("rounds to two decimals", () => {
    expect(formatBytes(1234)).toBe("1.21 KB");
    expect(formatBytes(1234567)).toBe("1.18 MB");
  });
});
