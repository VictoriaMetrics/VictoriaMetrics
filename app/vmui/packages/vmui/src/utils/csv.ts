export const formatValueToCSV= (value: string) =>
  (value.includes(",") || value.includes("\n") || value.includes("\""))
    ? "\"" + value.replace(/"/g, "\"\"") + "\""
    : value;
