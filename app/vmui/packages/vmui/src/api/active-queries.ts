export const getActiveQueries = (server: string): string =>
  `${server}/api/v1/status/active_queries`;
