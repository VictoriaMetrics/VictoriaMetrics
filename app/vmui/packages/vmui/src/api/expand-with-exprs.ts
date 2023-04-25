export const getExpandWithExprUrl = (server: string, query: string): string =>
  `${server}/expand-with-exprs?query=${query}&format=json`;
