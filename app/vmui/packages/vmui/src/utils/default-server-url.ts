import { getAppModeParams } from "./app-mode";

export const getDefaultServer = (tenantId?: string): string => {
  const { serverURL } = getAppModeParams();
  const url = serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
  if (tenantId) return replaceTenantId(url, tenantId);
  return "https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/prometheus";
  return url;
};

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;
  return serverUrl.replace(regexp, `$1${tenantId}/$4`);
};
