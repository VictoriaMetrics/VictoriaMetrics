import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
import { AppType } from "../types/appType";
import { getFromStorage } from "./storage";
const { REACT_APP_TYPE } = process.env;

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const storageURL = getFromStorage("SERVER_URL") as string;
  const logsURL = window.location.href.replace(/\/(select\/)?(vmui)\/.*/, "");
  const defaultURL = window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  const url = serverURL || storageURL || defaultURL;
  if (REACT_APP_TYPE === AppType.logs) return logsURL;
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};
