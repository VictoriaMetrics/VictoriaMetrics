export const getAccountIds = (server: string) =>
  `${server.replace(/^(.+)(\/select.+)/, "$1")}/admin/tenants`;
