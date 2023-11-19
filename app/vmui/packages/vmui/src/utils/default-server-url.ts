import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
import { AppType } from "../types/appType";
const { REACT_APP_TYPE } = process.env;

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const logsURL = window.location.href.replace(/\/(select\/)?(vmui)\/.*/, "");
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (REACT_APP_TYPE === AppType.logs) return logsURL;
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};
