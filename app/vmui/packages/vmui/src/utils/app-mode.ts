export interface AppParams {
  serverURL: string
}

const getAppModeParams = (): AppParams => {
  const dataParams = document.getElementById("root")?.dataset.params || "{}";
  return JSON.parse(dataParams);
};

const getAppModeEnable = (): boolean => !!Object.keys(getAppModeParams()).length;

export {getAppModeEnable, getAppModeParams};
