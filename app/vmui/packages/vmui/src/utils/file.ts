export const downloadFile = (data: Blob, filename: string) => {
  const link = document.createElement("a");
  const url = URL.createObjectURL(data);

  link.setAttribute("href", url);
  link.setAttribute("download", filename);

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

export const downloadCSV = (data: Record<string, string>[], filename: string) => {
  const getHeader = (data: Record<string, string>[]) => {
    const headersObj = data.reduce<Record<string, boolean>>((headers, row) => {
      Object.keys(row).forEach((key) => {
        if(key && !headers[key]){
          headers[key] = true;
        }
      });
      return headers;
    }, {});
    return Object.keys(headersObj);
  };

  const formatValueToCSV= (value: string) =>
    (value.includes(",") || value.includes("\n") || value.includes("\""))
      ? "\"" + value.replace(/"/g, "\"\"") + "\""
      : value;

  const convertToCSV = (data: Record<string, string>[]): string => {
    const header = getHeader(data);
    const rows = data.map(item =>
      header.map(fieldName => item[fieldName] ? formatValueToCSV(item[fieldName]): "").join(",")
    );
    return [header.map(formatValueToCSV).join(","), ...rows].join("\r\n");
  };

  const csvContent = convertToCSV(data);
  const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
  downloadFile(blob, filename);
};

export const downloadJSON = (data: string, filename: string) => {
  const blob = new Blob([data], { type: "application/json" });
  downloadFile(blob, filename);
};