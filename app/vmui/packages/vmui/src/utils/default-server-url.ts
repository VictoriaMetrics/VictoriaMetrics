import { getAppModeParams } from "./app-mode";

export const getDefaultServer = (): string => {
  return "https://play.victoriametrics.com/select/accounting/1/6a716b0f-38bc-4856-90ce-448fd713e3fe/prometheus/";
  const { serverURL } = getAppModeParams();
  return serverURL || window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus");
};
