export const getTopQueries = (server: string, topN: number | null, maxLifetime?: string) => (
  `${server}/api/v1/status/top_queries?topN=${topN || ""}&maxLifetime=${maxLifetime || ""}`
);
