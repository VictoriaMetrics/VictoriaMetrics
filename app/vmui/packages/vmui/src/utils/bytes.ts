const LOG_1024 = Math.log(1024);
const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

export const formatBytes = (bytes: number): string | null => {
  if (!Number.isFinite(bytes) || bytes < 0) return null;
  if (bytes === 0) return "0 B";

  const unitIndex = Math.min(
    Math.max(Math.floor(Math.log(bytes) / LOG_1024), 0),
    UNITS.length - 1
  );

  return `${parseFloat((bytes / 1024 ** unitIndex).toFixed(2))} ${UNITS[unitIndex]}`;
};
