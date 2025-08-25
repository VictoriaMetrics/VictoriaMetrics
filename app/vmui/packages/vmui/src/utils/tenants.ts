const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  return serverUrl.replace(regexp, `$1${tenantId}/$4`);
};

export const getTenantIdFromUrl = (url: string): string => {
  return url.match(regexp)?.[2] || "";
};

export const getUrlWithoutTenant = (url: string): string => {
  return url.replace(regexp, "");
};
