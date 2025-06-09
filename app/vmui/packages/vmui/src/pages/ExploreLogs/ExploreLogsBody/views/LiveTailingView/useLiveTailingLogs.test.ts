import { act, renderHook } from "@testing-library/preact";
import { useLiveTailingLogs } from "./useLiveTailingLogs";
import { vi } from "vitest";

vi.mock("../../../../../state/common/StateContext", () => ({
  useAppState: () => ({ serverUrl: "http://localhost:8080" }),
}));

vi.mock("../../../../../hooks/useTenant", () => ({
  useTenant: () => ({}),
}));

// Mock dependencies
const mockFetch = vi.fn();
global.fetch = mockFetch;

const createMockStreamResponse = (logs: string[], sendCount: number = 1) => ({
  ok: true,
  body: new ReadableStream({
    async start(controller) {
      for (let i = 0; i < sendCount; i++) {
        logs.forEach((log) => {
          controller.enqueue(new TextEncoder().encode(log + "\n"));
        });
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }

      controller.close();
    },
  }),
  text: async () => logs.join("\n"),
});

describe("useLiveTailingLogs", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
  });

  it("should start live tailing and process logs", async () => {
    const query = "*";
    const limit = 10;
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    mockFetch.mockResolvedValue(createMockStreamResponse(["{\"logs\":\"test log\"}"]));

    await act(async () => {
      const started = await result.current.startLiveTailing();
      expect(started).toBe(true);
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/select/logsql/tail",
      expect.objectContaining({
        method: "POST",
        body: new URLSearchParams({
          query: query.trim(),
        }),
      })
    );
  });

  it("should pause and resume live tailing", () => {
    const query = "*";
    const limit = 10;
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    act(() => {
      result.current.pauseLiveTailing();
    });

    expect(result.current.isPaused).toBe(true);

    act(() => {
      result.current.resumeLiveTailing();
    });

    expect(result.current.isPaused).toBe(false);
  });

  it("should stop live tailing", async () => {
    const query = "*";
    const limit = 10;
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    act(() => {
      result.current.stopLiveTailing();
    });

    expect(result.current.logs).toHaveLength(0);
  });

  it("should clear logs", () => {
    const query = "*";
    const limit = 10;
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    act(() => {
      result.current.clearLogs();
    });

    expect(result.current.logs).toEqual([]);
  });

  it("should handle errors during live tailing", async () => {
    const query = "*";
    const limit = 10;
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    mockFetch.mockRejectedValue(new Error("Network error"));

    await act(async () => {
      const started = await result.current.startLiveTailing();
      expect(started).toBe(false);
    });

    expect(result.current.error).toBe("Error: Network error");
    expect(result.current.logs).toHaveLength(0);
  });

  it("should process high load of logs incoming at 100k logs per second", async () => {
    const query = "*";
    const limit = 1000;
    const logCount = 10000; // High log rate
    const logs = Array.from({ length: logCount }, (_, i) => `{"log": "log message ${i}"}`);
    const { result } = renderHook(() => useLiveTailingLogs(query, limit));

    mockFetch.mockResolvedValue(createMockStreamResponse(logs, 7));

    await act(async () => {
      const started = await result.current.startLiveTailing();
      expect(started).toBe(true);
    });

    // Wait for logs to process
    await new Promise((resolve) => setTimeout(resolve, 7000));

    // Verify logs are limited and processed correctly
    expect(result.current.logs.length).toBeLessThanOrEqual(limit);
    // After setting flag isLimitedLogsPerUpdate when more than 200 logs received 5 times in a row,
    // we take only the last 200 logs, so we get 800 older logs (9200 - 9999) and 200 new logs (9800-9999)
    expect(result.current.logs[0].log).toStrictEqual("log message 9200");
    expect(result.current.logs[799].log).toStrictEqual("log message 9999");
    expect(result.current.isLimitedLogsPerUpdate).toBeTruthy();
  }, { timeout: 9000 });
});
