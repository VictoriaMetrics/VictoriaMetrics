export interface AppParams {
  serverURL?: string
  useTenantID?: boolean
  headerStyles?: {
    background?: string
    color?: string
  }
  palette?: {
    primary?: string
    secondary?: string
    error?: string
    warning?: string
    info?: string
    success?: string
  }
}

const getAppModeParams = (): AppParams => {
  const dataParams = document.getElementById("root")?.dataset.params || "{}";
  try {
    return JSON.parse(dataParams);
  } catch (e) {
    console.error(e);
    return {};
  }
};

const getAppModeEnable = (): boolean => !!Object.keys(getAppModeParams()).length;

export { getAppModeEnable, getAppModeParams };
