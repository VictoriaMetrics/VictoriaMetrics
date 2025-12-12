export const getGroupsUrl = (server: string, search: string, type: string, states: string[], maxGroups: number): string => {
  return `${server}/vmalert/api/v1/rules?datasource_type=prometheus&search=${search}&type=${type}&state=${states.join(",")}&group_limit=${maxGroups}`;
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
