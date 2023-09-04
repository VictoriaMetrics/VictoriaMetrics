export const getExpandWithExprUrl = (server: string, query: string): string =>
  `${server}/expand-with-exprs?query=${encodeURIComponent(query)}&format=json`;
