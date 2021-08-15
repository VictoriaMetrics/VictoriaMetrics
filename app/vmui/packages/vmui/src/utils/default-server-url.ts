export const getDefaultServer = (): string => {
  const {href} = window.location;
  const regexp = /^http.+\/vmui/;
  const [result] = href.match(regexp) || ["https://"];
  return result.replace("vmui", "prometheus");
};