import { getExportCSVDataUrl, getLabelsUrl } from "./query-range";
import { TimeParams } from "../types";
import { getCSVExportColumns } from "../utils/csv";

interface LabelsResponse {
  data?: string[];
}

export const fetchRawQueryCSVExport = async (
  serverUrl: string,
  query: string[],
  period: TimeParams,
  reduceMemUsage: boolean,
  fetchFn: typeof fetch = fetch,
): Promise<string> => {
  const labelsResponse = await fetchFn(getLabelsUrl(serverUrl, query, period));
  if (!labelsResponse.ok) {
    throw new Error(await labelsResponse.text());
  }

  const { data = [] } = (await labelsResponse.json()) as LabelsResponse;
  const columns = getCSVExportColumns(data);
  const format = columns.join(",");

  const response = await fetchFn(getExportCSVDataUrl(serverUrl, query, period, reduceMemUsage, format));
  if (!response.ok) {
    throw new Error(await response.text());
  }

  return await response.text();
};
