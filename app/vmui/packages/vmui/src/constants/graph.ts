import { GraphSize } from "../types";

export const MAX_QUERY_FIELDS = 4;
export const DEFAULT_MAX_SERIES = {
  table: 100,
  chart: 20,
  code: 1000,
};

export const GRAPH_SIZES: GraphSize[] = [
  {
    id: "small",
    height: () => window.innerHeight * 0.2
  },
  {
    id: "medium",
    isDefault: true,
    height: () => window.innerHeight * 0.4
  },
  {
    id: "large",
    height: () => window.innerHeight * 0.8
  },
];
