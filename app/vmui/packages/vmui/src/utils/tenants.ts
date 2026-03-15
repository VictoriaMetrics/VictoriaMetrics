const TENANT_REGEXP = /(\/select\/)(\d+(?::\d+)?)(\/.*)?$/;

export const replaceTenantId = (serverUrl: string, tenantId: string) => {
  return serverUrl.replace(TENANT_REGEXP, `$1${tenantId}$3`);
};

export const getTenantIdFromUrl = (url: string): string => {
  return url.match(TENANT_REGEXP)?.[2] ?? "";
};

export const getUrlWithoutTenant = (url: string): string => {
  return url.replace(TENANT_REGEXP, "");
};

export const updateBrowserUrlTenant = (tenantId: string) => {
  const base = `${window.location.origin}${window.location.pathname}${window.location.search}`;
  const nextBase = replaceTenantId(base, tenantId);

  const nextUrl = `${nextBase}${window.location.hash}`;
  window.history.replaceState(null, "", nextUrl);
};
