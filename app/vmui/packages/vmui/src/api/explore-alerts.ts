// vmalertSourceParam routes the request to a single vmalert from -vmalert.proxyURL.
// An empty source means that the request is sent to all the configured vmalerts.
const vmalertSourceParam = (source: string): string =>
  source ? `&vmalert_source=${encodeURIComponent(source)}` : "";

export const getGroupsUrl = (server: string, search: string, type: string, states: string[], maxGroups: number, source: string): string => {
  return `${server}/vmalert/api/v1/rules?datasource_type=prometheus&search=${encodeURIComponent(search)}&type=${encodeURIComponent(type)}&state=${states.map(encodeURIComponent).join(",")}&group_limit=${maxGroups}&extended_states=true${vmalertSourceParam(source)}`;
};

export const getItemUrl = (
  server: string,
  groupId: string,
  id: string,
  mode: string,
): string => {
  return `${server}/vmalert/api/v1/${mode}?group_id=${groupId}&${mode}_id=${id}`;
};

export const getGroupUrl = (
  server: string,
  id: string,
): string => {
  return `${server}/vmalert/api/v1/group?group_id=${id}`;
};

export const getNotifiersUrl = (server: string): string => {
  return `${server}/vmalert/api/v1/notifiers`;
};
