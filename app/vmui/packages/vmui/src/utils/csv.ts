export const formatValueToCSV= (value: string) =>
  (value.includes(",") || value.includes("\n") || value.includes("\""))
    ? "\"" + value.replace(/"/g, "\"\"") + "\""
    : value;

export const getCSVExportColumns = (labelNames: string[]) => {
  const labels = Array.from(new Set(labelNames.filter((label) => label && label !== "__name__"))).sort();
  return ["__name__", ...labels, "__value__", "__timestamp__:unix_ms"];
};
