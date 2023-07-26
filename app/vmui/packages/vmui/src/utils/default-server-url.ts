import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
const { REACT_APP_LOGS } = process.env;

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const logsURL = window.location.href.replace(/\/(select\/)?(vmui)\/.*/, "");
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (REACT_APP_LOGS) return logsURL;
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};
