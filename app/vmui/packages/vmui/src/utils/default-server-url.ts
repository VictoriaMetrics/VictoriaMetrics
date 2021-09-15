export const getDefaultServer = (): string => {
  const {href, protocol, host, pathname} = window.location;
  const regexp = /^http.+\/vmui/;
  const [result] = href.match(regexp) || [`${protocol}//${host}${pathname}prometheus`];
  return result.replace("vmui", "prometheus");
};