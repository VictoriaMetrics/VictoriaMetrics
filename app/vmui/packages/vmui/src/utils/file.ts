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

export const downloadCSV = (data: string, filename: string) => {
  const blob = new Blob([data],  { type: "text/csv;charset=utf-8;" });
  downloadFile(blob, filename);
};

export const downloadJSON = (data: string, filename: string) => {
  const blob = new Blob([data], { type: "application/json" });
  downloadFile(blob, filename);
};
