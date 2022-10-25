import {getAppModeParams} from "./app-mode";

export const getDefaultServer = (): string => {
  const {serverURL} = getAppModeParams();
  return serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
};
