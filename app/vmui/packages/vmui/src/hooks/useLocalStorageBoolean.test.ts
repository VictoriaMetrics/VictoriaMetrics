import { act, renderHook } from "@testing-library/preact";
import { useLocalStorageBoolean } from "./useLocalStorageBoolean";
import * as storageUtils from "../utils/storage";
import { Mock } from "vitest";
import { StorageKeys } from "../utils/storage";

vi.mock("../utils/storage");

const testStorageKey = "TEST_STORAGE_KEY" as StorageKeys;

describe("useLocalStorageBoolean", () => {
  const { getFromStorage, saveToStorage } = storageUtils;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("initializes with the value from localStorage", () => {
    const mockGetFromStorage = getFromStorage as Mock;
    mockGetFromStorage.mockReturnValueOnce(true);

    const { result } = renderHook(() => useLocalStorageBoolean(testStorageKey));

    expect(result.current[0]).toBe(true);
    expect(getFromStorage).toHaveBeenCalledWith(testStorageKey);
  });

  it("updates localStorage and state when setter is called", () => {
    const mockGetFromStorage = getFromStorage as Mock;
    mockGetFromStorage.mockReturnValueOnce(false);

    const { result } = renderHook(() => useLocalStorageBoolean(testStorageKey));

    act(() => {
      result.current[1](true);
    });

    expect(saveToStorage).toHaveBeenCalledWith(testStorageKey, true);
    expect(result.current[0]).toBe(false);
  });

  it("reacts to changes in localStorage by storage events", () => {
    const mockGetFromStorage = getFromStorage as Mock;
    mockGetFromStorage.mockReturnValueOnce(false);

    const { result } = renderHook(() => useLocalStorageBoolean(testStorageKey));

    // Simulate a storage event
    act(() => {
      mockGetFromStorage.mockReturnValueOnce(true);
      window.dispatchEvent(new StorageEvent("storage", { key: testStorageKey, newValue: "true" }));
    });

    expect(result.current[0]).toBe(true);
  });

  it("does not update state if the localStorage value remains the same", () => {
    const mockGetFromStorage = getFromStorage as Mock;
    mockGetFromStorage.mockReturnValueOnce(false);

    const { result } = renderHook(() => useLocalStorageBoolean(testStorageKey));

    act(() => {
      mockGetFromStorage.mockReturnValueOnce(false);
      window.dispatchEvent(new StorageEvent("storage", { key: testStorageKey, newValue: "false" }));
    });

    expect(result.current[0]).toBe(false);
  });
});
