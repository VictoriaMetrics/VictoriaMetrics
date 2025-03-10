/**
 * Unit tests for autocomplete utility functions
 */

import {
  getContext,
  getExprLastPart,
  getValueByContext,
} from "./autocompleteUtils";

// Mock QueryContextType enum
const QueryContextType = {
  empty: "empty",
  metricsql: "metricsql",
  label: "label",
  labelValue: "labelValue",
};

/**
 * Tests for getExprLastPart function
 */
function testExprLastPart() {
  const tests = [
    {
      input: "",
      expected: "",
    },
    {
      input: "metric",
      expected: "metric",
    },
    {
      input: "metric1 + metric2",
      expected: "metric2",
    },
    {
      input: "rate(proc",
      expected: "proc",
    },
    {
      input: "sum(rate(proc",
      expected: "proc",
    },
    {
      input: 'rate(metric{label="value",',
      expected: 'metric{label="value",',
    },
    {
      input: "quantile_over_time(0.9, me",
      expected: "me",
    },
    {
      input: "quantile_over_time(0.9, metric{label=",
      expected: "metric{label=",
    },
    {
      input: 'quantile_over_time(0.9, metric{label="value",',
      expected: 'metric{label="value",',
    },
    {
      input: 'quantile_over_time(0.9, metric{label="value"}',
      expected: "",
    },
    {
      input: "sum by (instance) (rate(proc",
      expected: "proc",
    },
    {
      input: "metric{label=",
      expected: "metric{label=",
    },
    {
      input: 'metric{label1="value1",label2=',
      expected: 'metric{label1="value1",label2=',
    },
    {
      input: "quantile_over_time(0.9, node_cpu",
      expected: "node_cpu",
    },
    {
      input: "sum(max_over_time(rate(node_cpu",
      expected: "node_cpu",
    },
    {
      input: "clamp_min(metric1, m",
      expected: "m",
    },
  ];

  let passedCount = 0;
  let failedCount = 0;

  console.log("Running getExprLastPart tests...\n");

  for (const test of tests) {
    const result = getExprLastPart(test.input);
    const passed = result === test.expected;

    if (passed) {
      passedCount++;
    } else {
      console.log(`FAIL: "${test.input}"`);
      console.log(`  Expected: "${test.expected}"`);
      console.log(`  Got:      "${result}"`);
      failedCount++;
    }
  }

  return failedCount === 0;
}

/**
 * Tests for getValueByContext function
 */
function testValueByContext() {
  const tests = [
    {
      input: "",
      expected: "",
    },
    {
      input: "metric",
      expected: "metric",
    },
    {
      input: "metric1 + metric2",
      expected: "metric2",
    },
    {
      input: "rate(proc",
      expected: "proc",
    },
    {
      input: "sum(rate(proc",
      expected: "proc",
    },
    {
      input: "quantile_over_time(0.9, node_cpu",
      expected: "node_cpu",
    },
    {
      input: "clamp_min(metric1, m",
      expected: "m",
    },
  ];

  let passedCount = 0;
  let failedCount = 0;

  console.log("Running getValueByContext tests...\n");

  for (const test of tests) {
    const result = getValueByContext(test.input);
    const passed = result === test.expected;

    if (passed) {
      passedCount++;
    } else {
      console.log(`FAIL: "${test.input}"`);
      console.log(`  Expected: "${test.expected}"`);
      console.log(`  Got:      "${result}"`);
      failedCount++;
    }
  }

  return failedCount === 0;
}

/**
 * Tests for getContext function
 */
function testContext() {
  const tests = [
    {
      input: { beforeCursor: "" },
      expected: QueryContextType.empty,
    },
    {
      input: { beforeCursor: "metric" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "rate(proc" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "sum(rate(proc" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "sum by (instance) (rate(proc" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "metric{" },
      expected: QueryContextType.label,
    },
    {
      input: { beforeCursor: "metric{label=" },
      expected: QueryContextType.labelValue,
      metric: "metric",
      label: "label",
    },
    {
      input: { beforeCursor: 'metric{label1="value1",label2=' },
      expected: QueryContextType.labelValue,
      metric: "metric",
      label: "label2",
    },
    {
      input: { beforeCursor: "sum by (" },
      expected: QueryContextType.label,
    },
    {
      input: { beforeCursor: "quantile_over_time(0.9, node_cpu" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "sum(max_over_time(rate(node_cpu" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "clamp_min(metric1, m" },
      expected: QueryContextType.metricsql,
    },
    {
      input: { beforeCursor: "rate(node_cpu_seconds_total)" },
      expected: QueryContextType.empty,
    },
  ];

  let passedCount = 0;
  let failedCount = 0;

  console.log("Running getContext tests...\n");

  for (const test of tests) {
    const { beforeCursor } = test.input;
    const metric = test.metric || "";
    const label = test.label || "";
    const result = getContext(beforeCursor, metric, label);
    const passed = result === test.expected;

    if (passed) {
      passedCount++;
    } else {
      console.log(`FAIL: ${test.input.beforeCursor}`);
      if (metric) console.log(`  Metric:   "${metric}"`);
      if (label) console.log(`  Label:    "${label}"`);
      console.log(`  Expected: "${test.expected}"`);
      console.log(`  Got:      "${result}"`);
      failedCount++;
    }
  }

  return failedCount === 0;
}

/**
 * Run all tests
 */
function runAllTests() {
  const resultExprLastPart = testExprLastPart();
  const resultValueByContext = testValueByContext();
  const resultContext = testContext();

  if (!resultExprLastPart || !resultValueByContext || !resultContext) {
    process.exit(1);
  }
}

// Run tests
runAllTests();
