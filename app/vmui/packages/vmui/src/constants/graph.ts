import { GraphSize, SeriesItemStatsFormatted } from "../types";

export const MAX_QUERY_FIELDS = 10;
export const MAX_QUERIES_HISTORY = 25;
export const DEFAULT_MAX_SERIES = {
  table: 100,
  chart: 20,
  code: 1000,
};

export const GRAPH_SIZES: GraphSize[] = [
  {
    id: "small",
    isDefault: true,
    height: () => window.innerHeight * 0.2
  },
  {
    id: "medium",
    height: () => window.innerHeight * 0.4
  },
  {
    id: "large",
    height: () => window.innerHeight * 0.8
  },
];

export const STATS_ORDER: (keyof SeriesItemStatsFormatted)[] = ["min", "median", "max"];
