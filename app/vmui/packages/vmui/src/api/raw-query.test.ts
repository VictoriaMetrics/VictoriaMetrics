import { describe, expect, it, vi } from "vitest";
import { fetchRawQueryCSVExport } from "./raw-query";

describe("fetchRawQueryCSVExport", () => {
  it.skip("requests all label columns before exporting CSV data", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: ["job", "__name__", "instance"] }),
      })
      .mockResolvedValueOnce({
        ok: true,
        text: async () => "up,localhost:9100,node_exporter,1,1710000000000",
      });

    const result = await fetchRawQueryCSVExport(
      "http://localhost:8428",
      ["up"],
      { start: 1710000000, end: 1710000300, step: "15s", date: "2024-03-09T16:05:00Z" },
      false,
      fetchMock as unknown as typeof fetch,
    );

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8428/api/v1/labels?start=1710000000&end=1710000300&match%5B%5D=up");
    expect(fetchMock.mock.calls[1][0]).toBe("http://localhost:8428/api/v1/export/csv?start=1710000000&end=1710000300&match%5B%5D=up&format=__name__%2Cinstance%2Cjob%2C__value__%2C__timestamp__%3Aunix_ms");
    expect(result).toBe("up,localhost:9100,node_exporter,1,1710000000000");
  });
});
