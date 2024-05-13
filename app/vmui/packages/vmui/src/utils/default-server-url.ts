import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
import { AppType } from "../types/appType";
import { getFromStorage } from "./storage";
const { REACT_APP_TYPE } = process.env;

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const storageURL = getFromStorage("SERVER_URL") as string;
  const logsURL = window.location.href.replace(/\/(select\/)?(vmui)\/.*/, "");
  const anomalyURL = `${window.location.origin}${window.location.pathname}`;
  const defaultURL = window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  const url = serverURL || storageURL || defaultURL;

  switch (REACT_APP_TYPE) {
    case AppType.logs:
      return logsURL;
    case AppType.anomaly:
      return storageURL || anomalyURL;
    default:
      return tenantId ? replaceTenantId(url, tenantId) : url;
  }
};
