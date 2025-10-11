import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
import { APP_TYPE, AppType } from "../constants/appType";
import { getFromStorage } from "./storage";

export const getDefaultURL = (u: string) => {
  return u.replace(/(\/(?:prometheus\/)?(?:graph|vmui)\/.*|\/#\/.*)/, "").replace(/(\/select\/[^/]+)$/, "$1/prometheus");
};

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const storageURL = getFromStorage("SERVER_URL") as string;
  const anomalyURL = `${window.location.origin}${window.location.pathname.replace(/^\/vmui/, "")}`;
  const defaultURL = getDefaultURL(window.location.href);
  const url = serverURL || storageURL || defaultURL;

  switch (APP_TYPE) {
    case AppType.vmanomaly:
      return storageURL || anomalyURL;
    default:
      return tenantId ? replaceTenantId(url, tenantId) : url;
  }
};
