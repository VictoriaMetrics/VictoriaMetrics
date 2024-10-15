export const getDownsamplingFiltersDebug = (server: string, flags: string, metrics: string): string => {
  const params = [
    `flags=${encodeURIComponent(flags)}`,
    `metrics=${encodeURIComponent(metrics)}`
  ];
  return `${server}/downsampling-filters-debug?${params.join("&")}`;
};
