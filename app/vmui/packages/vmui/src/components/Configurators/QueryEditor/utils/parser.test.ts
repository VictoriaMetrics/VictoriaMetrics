import { describe, it, expect } from "vitest";
import {
  splitByCursor,
  extractMetric,
  extractCurrentLabel,
  extractLabelMatchers,
} from "./parser";

describe("splitByCursor", () => {
  it("splits by caret when selection is collapsed", () => {
    const res = splitByCursor("abcdef", [2, 2]);
    expect(res).toEqual({ beforeCursor: "ab", afterCursor: "cdef" });
  });

  it("returns whole value as beforeCursor when selection is not collapsed", () => {
    const res = splitByCursor("abcdef", [1, 3]);
    expect(res).toEqual({ beforeCursor: "abcdef", afterCursor: "" });
  });

  it("handles caret at 0", () => {
    const res = splitByCursor("abc", [0, 0]);
    expect(res).toEqual({ beforeCursor: "", afterCursor: "abc" });
  });

  it("handles caret at end", () => {
    const res = splitByCursor("abc", [3, 3]);
    expect(res).toEqual({ beforeCursor: "abc", afterCursor: "" });
  });

  it("treats reversed selection as non-collapsed (browser may return [end,start])", () => {
    const res = splitByCursor("abcdef", [4, 2]);
    expect(res).toEqual({ beforeCursor: "abcdef", afterCursor: "" });
  });
});

describe("extractMetric", () => {
  it("extracts metric from plain selector", () => {
    expect(extractMetric("kube_pod_info{job=\"x\"}")).toBe("kube_pod_info");
  });

  it("extracts metric from plain expr with leading spaces", () => {
    expect(extractMetric("   http_requests_total")).toBe("http_requests_total");
  });

  it("extracts metric from expr with braces right after metric", () => {
    expect(extractMetric("foo_bar{a=\"b\"}")).toBe("foo_bar");
  });

  it("extracts metric before grouping modifiers (by/without/on/ignoring)", () => {
    expect(extractMetric("sum(kube_pod_info) by (pod)")).toBe("kube_pod_info");
    expect(extractMetric("sum(kube_pod_info) without (pod)")).toBe("kube_pod_info");
    expect(extractMetric("sum(kube_pod_info) on (pod)")).toBe("kube_pod_info");
    expect(extractMetric("sum(kube_pod_info) ignoring (pod)")).toBe("kube_pod_info");
  });

  it("returns empty string when no metric found", () => {
    expect(extractMetric("{job=\"x\"}")).toBe("");
    expect(extractMetric("")).toBe("");
    expect(extractMetric("()")).toBe("");
  });
});

describe("extractCurrentLabel", () => {
  it("returns last label before operator", () => {
    expect(extractCurrentLabel("metric{job=\"foo\", instance=\"bar\"}")).toBe(
      "instance"
    );
  });

  it("supports spaces around operator", () => {
    expect(extractCurrentLabel("metric{job=\"foo\", instance = \"bar\"}")).toBe(
      "instance"
    );
  });

  it("supports regexp operators", () => {
    expect(extractCurrentLabel("metric{pod=~\"api-.*\",namespace=\"dev\"}")).toBe(
      "namespace"
    );
  });

  it("supports label chars : - . /", () => {
    expect(extractCurrentLabel("m{foo-bar.baz/qux=\"1\"}")).toBe("foo-bar.baz/qux");
  });

  it("returns empty string when no label pattern", () => {
    expect(extractCurrentLabel("metric{}").trim()).toBe("");
    expect(extractCurrentLabel("metric")).toBe("");
  });
});

describe("extractLabelMatchers", () => {
  it("returns all matchers (quoted only)", () => {
    const expr = "metric{job=\"foo\", instance=\"bar\"}";
    expect(extractLabelMatchers(expr)).toEqual(["job=\"foo\"", "instance=\"bar\""]);
  });

  it("keeps original spacing", () => {
    const expr = "metric{ job = \"foo\" , instance = \"bar\" }";
    expect(extractLabelMatchers(expr)).toEqual(["job = \"foo\"", "instance = \"bar\""]);
  });

  it("supports !=, =~, !~", () => {
    const expr = "m{env!=\"prod\",pod=~\"api-.*\",zone!~\"eu-.*\"}";
    expect(extractLabelMatchers(expr)).toEqual([
      "env!=\"prod\"",
      "pod=~\"api-.*\"",
      "zone!~\"eu-.*\"",
    ]);
  });

  it("excludes only the specified currentLabel matcher (exact label, not prefix)", () => {
    const expr = "m{job=\"foo\", instance=\"bar\", pod=~\"api-.*\"}";
    expect(extractLabelMatchers(expr, "instance")).toEqual([
      "job=\"foo\"",
      "pod=~\"api-.*\"",
    ]);
  });

  it("does not exclude other labels that share a prefix with currentLabel", () => {
    const expr = "m{instance=\"bar\", insight=\"x\"}";
    expect(extractLabelMatchers(expr, "insight")).toEqual(["instance=\"bar\""]);
  });

  it("excludes currentLabel matcher even with spaces around operator", () => {
    const expr = "m{job=\"foo\", instance = \"bar\"}";
    expect(extractLabelMatchers(expr, "instance")).toEqual(["job=\"foo\""]);
  });

  it("returns [] when no matchers", () => {
    expect(extractLabelMatchers("m{}")).toEqual([]);
    expect(extractLabelMatchers("m")).toEqual([]);
  });

  it("does not include unclosed quotes", () => {
    const expr = "m{job=\"foo\", instance=\"ba";
    expect(extractLabelMatchers(expr)).toEqual(["job=\"foo\""]);
  });
});
