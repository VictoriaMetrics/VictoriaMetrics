export const getDefaultServer = (): string => {
  return window.location.href.replace(/\/(?:prometheus\/)?(?:graph|vmui)\/.*/, "/prometheus/");
};
