import { getUrlWithoutTenant } from "../utils/tenants";
export const getAccountIds = (server: string) => `${getUrlWithoutTenant(server)}/admin/tenants`;
