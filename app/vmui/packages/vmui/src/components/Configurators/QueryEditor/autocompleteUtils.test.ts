import { describe, it, expect } from "vitest";
import { getContext, getExprLastPart, getValueByContext } from "./autocompleteUtils";

// Mock QueryContextType enum
const QueryContextType = {
  empty: "empty",
  metricsql: "metricsql",
  label: "label",
  labelValue: "labelValue",
};

describe("autocompleteUtils", () => {
  describe("getExprLastPart", () => {
    const tests = [
      { input: "", expected: "" },
      { input: "metric", expected: "metric" },
      { input: "metric1 + metric2", expected: "metric2" },
      { input: "rate(proc", expected: "proc" },
      { input: "sum(rate(proc", expected: "proc" },
      { input: "rate(metric{label=\"value\",", expected: "metric{label=\"value\"," },
      { input: "quantile_over_time(0.9, me", expected: "me" },
      { input: "quantile_over_time(0.9, metric{label=", expected: "metric{label=" },
      { input: "quantile_over_time(0.9, metric{label=\"value\",", expected: "metric{label=\"value\"," },
      { input: "quantile_over_time(0.9, metric{label=\"value\"}", expected: "" },
      { input: "sum by (instance) (rate(proc", expected: "proc" },
      { input: "metric{label=", expected: "metric{label=" },
      { input: "metric{label1=\"value1\",label2=", expected: "metric{label1=\"value1\",label2=" },
      { input: "quantile_over_time(0.9, node_cpu", expected: "node_cpu" },
      { input: "sum(max_over_time(rate(node_cpu", expected: "node_cpu" },
      { input: "clamp_min(metric1, m", expected: "m" },
    ];

    tests.forEach(({ input, expected }) => {
      it(`should return "${expected}" for input "${input}"`, () => {
        const result = getExprLastPart(input);
        expect(result).toBe(expected);
      });
    });
  });

  describe("getValueByContext", () => {
    const tests = [
      { input: "", expected: "" },
      { input: "metric", expected: "metric" },
      { input: "metric1 + metric2", expected: "metric2" },
      { input: "rate(proc", expected: "proc" },
      { input: "sum(rate(proc", expected: "proc" },
      { input: "quantile_over_time(0.9, node_cpu", expected: "node_cpu" },
      { input: "clamp_min(metric1, m", expected: "m" },
    ];

    tests.forEach(({ input, expected }) => {
      it(`should return "${expected}" for input "${input}"`, () => {
        const result = getValueByContext(input);
        expect(result).toBe(expected);
      });
    });
  });

  describe("getContext", () => {
    const tests = [
      { input: { beforeCursor: "" }, expected: QueryContextType.empty },
      { input: { beforeCursor: "metric" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "rate(proc" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "sum(rate(proc" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "sum by (instance) (rate(proc" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "metric{" }, expected: QueryContextType.label },
      { input: { beforeCursor: "metric{label=" }, expected: QueryContextType.labelValue, metric: "metric", label: "label" },
      { input: { beforeCursor: "metric{label1=\"value1\",label2=" }, expected: QueryContextType.labelValue, metric: "metric", label: "label2" },
      { input: { beforeCursor: "sum by (" }, expected: QueryContextType.label },
      { input: { beforeCursor: "quantile_over_time(0.9, node_cpu" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "sum(max_over_time(rate(node_cpu" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "clamp_min(metric1, m" }, expected: QueryContextType.metricsql },
      { input: { beforeCursor: "rate(node_cpu_seconds_total)" }, expected: QueryContextType.empty },
    ];

    tests.forEach((test) => {
      const { beforeCursor } = test.input;
      const metric = test.metric || "";
      const label = test.label || "";
      it(`should return "${test.expected}" for input "${beforeCursor}"`, () => {
        const result = getContext(beforeCursor, metric, label);
        expect(result).toBe(test.expected);
      });
    });
  });
});
