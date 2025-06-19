import { expect } from "vitest";
import { generateQuery } from "./utils";
import { ContextType } from "./types";

describe("utils", () => {
  describe("_time", () => {
    it("should return the trimmed value by `-`", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "_stream:{type=\"WatchEvent\"}",
        contextType: ContextType.FilterValue,
        filterName: "_time",
        query: "_stream:{type=\"WatchEvent\"} _time:2025-04-1",
        valueAfterCursor: "",
        valueBeforeCursor: "_time=2025-04-1",
        valueContext: "2025-04-1"
      })).toStrictEqual("_stream:{type=\"WatchEvent\"} _time:2025-04");
    });

    it("should return the trimmed value by `:` if char `-` also exist in the query", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "_stream:{type=\"WatchEvent\"}",
        contextType: ContextType.FilterValue,
        filterName: "_time",
        query: "_stream:{type=\"WatchEvent\"} _time:2025-04-10T23:45:5",
        valueAfterCursor: "",
        valueBeforeCursor: "_time=2025-04-10T23:45:5",
        valueContext: "2025-04-10T23:45:5"
      })).toStrictEqual("_stream:{type=\"WatchEvent\"} _time:2025-04-10T23:45");
    });

    it("should return default `*` instead of -time filter", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "_stream:{type=\"WatchEvent\"}",
        contextType: ContextType.FilterValue,
        filterName: "_time",
        query: "_stream:{type=\"WatchEvent\"} _time:202",
        valueAfterCursor: "",
        valueBeforeCursor: "_time=202",
        valueContext: "202"
      })).toStrictEqual("_stream:{type=\"WatchEvent\"} *");
    });
  });

  describe("_stream", () => {
    it("should add regexp to filter value", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "",
        contextType: ContextType.FilterValue,
        filterName: "_stream",
        query: "_stream:{type=\"WatchEve",
        valueAfterCursor: "",
        valueBeforeCursor: "_stream:{type=\"WatchEve",
        valueContext: "{type=\"WatchEve"
      })).toStrictEqual("_stream:{type=~\"WatchEve.*\"}");
    });

    it("should add regexp to filter value if cursor in the middle of value", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "",
        contextType: ContextType.FilterValue,
        filterName: "_stream",
        query: "_stream:{type=\"WatchEve\"}",
        valueAfterCursor: "",
        valueBeforeCursor: "_stream:{type=\"WatchEve",
        valueContext: "{type=\"WatchEve"
      })).toStrictEqual("_stream:{type=~\"WatchEve.*\"}");
    });

    it("should return * if do not have value after =", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "",
        contextType: ContextType.FilterValue,
        filterName: "_stream",
        query: "_stream:{type=",
        valueAfterCursor: "",
        valueBeforeCursor: "_stream:{type=",
        valueContext: "{type="
      })).toStrictEqual("*");
    });
  });

  it("_msg", () => {
    expect(generateQuery({
      queryBeforeIncompleteFilter: "_stream:{type=\"WatchEvent\"}",
      contextType: ContextType.FilterValue,
      filterName: "_msg",
      query: "_stream:{type=\"WatchEvent\"} _msg:453",
      valueAfterCursor: "",
      valueBeforeCursor: "_msg:453",
      valueContext: "453"
    })).toStrictEqual("_stream:{type=\"WatchEvent\"} *");
  });

  it("_stream_id", () => {
    expect(generateQuery({
      queryBeforeIncompleteFilter: "_stream:{type=\"WatchEvent\"}",
      contextType: ContextType.FilterValue,
      filterName: "_stream_id",
      query: "_stream:{type=\"WatchEvent\"} _stream_id:453",
      valueAfterCursor: "",
      valueBeforeCursor: "_stream_id:453",
      valueContext: "453"
    })).toStrictEqual("_stream:{type=\"WatchEvent\"} *");
  });

  describe("other fields", () => {
    it("should add prefix filter to other type of field names", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "",
        contextType: ContextType.FilterValue,
        filterName: "repo.name",
        query: "repo.name:Victori",
        valueAfterCursor: "",
        valueBeforeCursor: "repo.name:Victori",
        valueContext: "Victori"
      })).toStrictEqual("repo.name:Victori*");
    });

    it("should add prefix filter to other type of field names with escaped via double quote", () => {
      expect(generateQuery({
        queryBeforeIncompleteFilter: "",
        contextType: ContextType.FilterValue,
        filterName: "repo.name",
        query: "repo.name:\"Victori",
        valueAfterCursor: "",
        valueBeforeCursor: "repo.name:\"Victori",
        valueContext: "Victori"
      })).toStrictEqual("repo.name:Victori*");
    });
  });
});