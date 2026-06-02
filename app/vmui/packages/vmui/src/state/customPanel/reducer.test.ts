import { afterEach, describe, expect, it, vi, type Mock } from "vitest";
import { getFromStorage, saveToStorage } from "../../utils/storage";

vi.mock("../../utils/storage", () => ({
  getFromStorage: vi.fn(),
  saveToStorage: vi.fn(),
}));

describe("customPanel reducer", () => {
  afterEach(() => {
    vi.resetAllMocks();
    vi.resetModules();
  });

  it("persists reduceMemUsage under its own storage key", async () => {
    const { reducer, initialCustomPanelState } = await import("./reducer");

    reducer(initialCustomPanelState, { type: "TOGGLE_REDUCE_MEM_USAGE" });

    expect(saveToStorage).toHaveBeenCalledWith("REDUCE_MEM_USAGE", true);
    expect(saveToStorage).not.toHaveBeenCalledWith("TABLE_COMPACT", true);
  });

  it("hydrates reduceMemUsage from storage", async () => {
    const getFromStorageMock = getFromStorage as Mock;
    getFromStorageMock.mockImplementation((key: string) => {
      if (key === "REDUCE_MEM_USAGE") return true;
      return undefined;
    });

    const { initialCustomPanelState } = await import("./reducer");

    expect(initialCustomPanelState.reduceMemUsage).toBe(true);
  });
});
