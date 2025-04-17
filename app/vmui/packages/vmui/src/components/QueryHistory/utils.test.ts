import { describe, it, expect, vi, Mock, afterEach } from "vitest";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import { getUpdatedHistory, setQueriesToStorage } from "./utils";
import { MAX_QUERIES_HISTORY, MAX_QUERY_FIELDS } from "../../constants/graph";

vi.mock("../../utils/storage", () => ({
  getFromStorage: vi.fn(),
  saveToStorage: vi.fn(),
}));

describe("utils", () => {
  afterEach(() => {
    vi.resetAllMocks();
  });
  describe("setQueriesToStorage", () => {
    it("should not change QUERY_HISTORY ", () => {
      const getFromStorageMock = getFromStorage as Mock;
      const saveToStorageMock = saveToStorage as Mock;
      getFromStorageMock.mockReturnValue(JSON.stringify({
        "QUERY_HISTORY": [],
      }));

      setQueriesToStorage("LOGS_QUERY_HISTORY", []);
      expect(saveToStorageMock).toHaveBeenCalledWith(
        "LOGS_QUERY_HISTORY",
        "{\"QUERY_HISTORY\":[[]]}"
      );
    });

    it("should not change QUERY_HISTORY cause add the same query", () => {
      const getFromStorageMock = getFromStorage as Mock;
      const saveToStorageMock = saveToStorage as Mock;
      getFromStorageMock.mockReturnValue(JSON.stringify({
        "QUERY_HISTORY": [["first_query"]],
      }));

      setQueriesToStorage("LOGS_QUERY_HISTORY", [{ index: 0, values: ["first_query"] }]);
      expect(saveToStorageMock).toHaveBeenCalledWith(
        "LOGS_QUERY_HISTORY",
        "{\"QUERY_HISTORY\":[[\"first_query\"]]}"
      );
    });

    it("should add new query to the first position to QUERY_HISTORY", () => {
      const getFromStorageMock = getFromStorage as Mock;
      const saveToStorageMock = saveToStorage as Mock;
      getFromStorageMock.mockReturnValue(JSON.stringify({
        "QUERY_HISTORY": [["first_query"]],
      }));

      setQueriesToStorage("LOGS_QUERY_HISTORY", [{ index: 0, values: ["new_query"] }]);
      expect(saveToStorageMock).toHaveBeenCalledWith(
        "LOGS_QUERY_HISTORY",
        "{\"QUERY_HISTORY\":[[\"new_query\",\"first_query\"]]}"
      );
    });

    it("should limit the QUERY_HISTORY if add extra query", () => {
      const getFromStorageMock = getFromStorage as Mock;
      const saveToStorageMock = saveToStorage as Mock;
      const maxQueries = MAX_QUERIES_HISTORY * MAX_QUERY_FIELDS;
      const currentHistory = (new Array(maxQueries)).fill(1).map((_, i) => `${i}_query`);
      getFromStorageMock.mockReturnValue(JSON.stringify({
        "QUERY_HISTORY": [currentHistory],
      }));

      setQueriesToStorage("LOGS_QUERY_HISTORY", [{ index: 0, values: ["extra_query"] }]);

      const calls = saveToStorageMock.mock.calls;
      const firstCallArgs = calls[0];
      expect(firstCallArgs[0]).toStrictEqual("LOGS_QUERY_HISTORY");
      const savedQueries = JSON.parse(firstCallArgs[1]);
      expect(savedQueries["QUERY_HISTORY"][0][0]).toStrictEqual("extra_query");
      expect(savedQueries["QUERY_HISTORY"][0].length).toStrictEqual(maxQueries);
    });
  });

  describe("getUpdatedHistory", () => {
    it("should add new query to the end of array", () => {
      const updatedHistory = getUpdatedHistory("new_query", {
        index: 2,
        values: ["first_query", "second_query"]
      });
      expect(updatedHistory).toStrictEqual({
        index: 2,
        values: [
          "first_query",
          "second_query",
          "new_query",
        ],
      });
    });

    it("should not add new query if the last query is the same", () => {
      const updatedHistory = getUpdatedHistory("new_query", {
        index: 2,
        values: ["first_query", "new_query"]
      });
      expect(updatedHistory).toStrictEqual({
        index: 1,
        values: [
          "first_query",
          "new_query",
        ],
      });
    });

    it("should remove the first query if the maximum number of query is reached", () => {
      const maxQueries = MAX_QUERIES_HISTORY * MAX_QUERY_FIELDS;
      const values = (new Array(maxQueries)).fill(1).map((_, i) => `${i}_query`);
      const updatedHistory = getUpdatedHistory("new_query", {
        index: 2,
        values: values
      });
      expect(updatedHistory.index).toStrictEqual(maxQueries);
      expect(updatedHistory.values.length).toStrictEqual(maxQueries);
      expect(updatedHistory.values[0]).toStrictEqual("1_query");
      expect(updatedHistory.values[updatedHistory.values.length - 1]).toStrictEqual("new_query");
    });
  });
});