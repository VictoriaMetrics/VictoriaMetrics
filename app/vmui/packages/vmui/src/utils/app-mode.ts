export interface AppParams {
  serverURL: string
  headerStyles?: {
    background?: string
    color?: string
  }
  palette?: {
    primary?: string
    secondary?: string
    error?: string
  }
}

const getAppModeParams = (): AppParams => {
  const dataParams = document.getElementById("root")?.dataset.params || "{}";
  return JSON.parse(dataParams);
};

const getAppModeEnable = (): boolean => !!Object.keys(getAppModeParams()).length;

export {getAppModeEnable, getAppModeParams};
