import { getAppModeParams } from "./app-mode";

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;
  return serverUrl.replace(regexp, `$1${tenantId}/$4`);
};
