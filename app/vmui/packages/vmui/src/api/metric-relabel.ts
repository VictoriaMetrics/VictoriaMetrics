export const getMetricRelabelDebug = (server: string, configs: string, metric: string): string => {
  const params = [
    "format=json",
    `relabel_configs=${encodeURIComponent(configs)}`,
    `metric=${encodeURIComponent(metric)}`
  ];
  return `${server}/metric-relabel-debug?${params.join("&")}`;
};
