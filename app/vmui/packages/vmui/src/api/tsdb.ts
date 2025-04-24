export interface CardinalityRequestsParams {
  topN: number,
  match: string | null,
  date: string | null,
  focusLabel: string | null,
}

export const getCardinalityInfo = (server: string, requestsParam: CardinalityRequestsParams) => {
  const match = requestsParam.match ? "&match[]=" + encodeURIComponent(requestsParam.match) : "";
  const focusLabel = requestsParam.focusLabel ? "&focusLabel=" + encodeURIComponent(requestsParam.focusLabel) : "";
  return `${server}/api/v1/status/tsdb?topN=${requestsParam.topN}&date=${requestsParam.date}${match}${focusLabel}`;
};

interface MetricNamesStatsParams {
  limit?: number,
  le?: number, // less than or equal
  match_pattern?: string, // a regex pattern to match metric names
}

export const getMetricNamesStats = (server: string, params: MetricNamesStatsParams) => {
  const searchParams = new URLSearchParams(
    Object.entries(params).filter(([_, value]) => value != null)
  ).toString();

  console.log("searchParams", searchParams);

  return `${server}/api/v1/status/metric_names_stats?${searchParams}`;
};

