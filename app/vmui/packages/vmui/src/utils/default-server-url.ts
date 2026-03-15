import { getAppModeParams } from "./app-mode";
import { getFromStorage } from "./storage";

export const getDefaultURL = (u: string) => {
  return u.replace(/(\/(?:prometheus\/)?(?:graph|vmui)\/.*|\/#\/.*)/, "/prometheus");
};

export const getDefaultServer = (): string => {
  const { serverURL } = getAppModeParams();
  const storageURL = getFromStorage("SERVER_URL") as string;
  const defaultURL = getDefaultURL(window.location.href);
  return serverURL || storageURL || defaultURL;
};
