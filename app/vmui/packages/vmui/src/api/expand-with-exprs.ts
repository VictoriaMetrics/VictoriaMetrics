export const getExpandWithExprUrl = (server: string, query: string): string =>
  `${server}/expand-with-exprs?json=true&query=${query}`;
