import { getAppModeParams } from "./app-mode";
const { REACT_APP_LOGS } = process.env;

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const logsURL = `${window.location.protocol}//${window.location.host}`;
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (REACT_APP_LOGS) return logsURL;
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;
  return serverUrl.replace(regexp, `$1${tenantId}/$4`);
};
