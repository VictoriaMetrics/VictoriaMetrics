export const escapeRegexp = (s: string) => {
  // taken from https://stackoverflow.com/a/3561711/274937
  return s.replace(/[/\-\\^$*+?.()|[\]{}]/g, "\\$&");
};

export const escapeDoubleQuotes = (s: string) => {
  return JSON.stringify(s).slice(1,-1);
};

export const hasUnclosedQuotes = (str: string) => {
  const matches = str.match(/"/g);
  return matches ? matches.length % 2 !== 0 : false;
};
