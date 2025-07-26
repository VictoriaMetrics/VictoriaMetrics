export const getGroupsUrl = (server: string, ruleType: string): string => {
  let groupUrl = `${server}/api/v1/rules`;
  if (ruleType) {
    groupUrl = `${groupUrl}?type=${ruleType}`;
  }
  return groupUrl;
};

export const getNotifiersUrl = (server: string): string => {
  return `${server}/api/v1/notifiers`;
};
