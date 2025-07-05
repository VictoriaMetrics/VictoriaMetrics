import { APP_TYPE, AppType } from "../constants/appType";

const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  return serverUrl.replace(regexp, `$1${tenantId}/$4`);
};

export const getTenantIdFromUrl = (url: string): string => {
  return url.match(regexp)?.[2] || "";
};

export const getUrlWithoutTenant = (server: string): string => {
  switch (APP_TYPE) {
    case AppType.victorialogs:
      return server.replace(/^(.*)(\/select)/, "$1");
    default:
      return server.replace(/^(.*)(\/select\/[^/]+)/, "$1");
  }
};
