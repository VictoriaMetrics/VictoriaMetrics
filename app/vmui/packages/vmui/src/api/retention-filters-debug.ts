export const getRetentionFiltersDebug = (server: string, flags: string, metrics: string): string => {
  const params = [
    `flags=${encodeURIComponent(flags)}`,
    `metrics=${encodeURIComponent(metrics)}`
  ];
  return `${server}/retention-filters-debug?${params.join("&")}`;
};
