import { getUrlWithoutTenant } from "../utils/tenants";
import { TimeParams } from "../types";

export const getAccountIds = (server: string, period: TimeParams) => {
  const params = new URLSearchParams({
    start: period.start.toString(),
    end: period.end.toString(),
  });

  return `${getUrlWithoutTenant(server)}/admin/tenants?${params}`;
};
