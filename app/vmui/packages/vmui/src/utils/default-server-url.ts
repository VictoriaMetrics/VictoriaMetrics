import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";
import { APP_TYPE, AppType } from "../constants/appType";
import { getFromStorage } from "./storage";

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const storageURL = getFromStorage("SERVER_URL") as string;
  const defaultURL = window.location.href.replace(/\/(vmui|#)\/.*/, "");
  const alertURL = window.location.href.replace(/\/(vmalert|#)\/.*/, "");
  const url = serverURL || storageURL || defaultURL;

  switch (APP_TYPE) {
    case AppType.victorialogs:
      return defaultURL;
    case AppType.vmanomaly:
      return storageURL || defaultURL;
    case AppType.vmalert:
      return storageURL || alertURL;
    default:
      return tenantId ? replaceTenantId(url, tenantId) : url;
  }
};
