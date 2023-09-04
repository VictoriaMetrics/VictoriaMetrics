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

