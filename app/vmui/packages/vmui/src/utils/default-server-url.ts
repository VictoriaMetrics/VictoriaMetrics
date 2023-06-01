import { getAppModeParams } from "./app-mode";
import { replaceTenantId } from "./tenants";

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (tenantId) return replaceTenantId(url, tenantId);
  return url;
};
