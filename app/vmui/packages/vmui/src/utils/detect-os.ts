const desktopOs = {
  windows: "Windows",
  mac: "Mac OS",
  linux: "Linux"
};

export const getOs = () : string => {
  return Object.values(desktopOs).find(os => navigator.userAgent.indexOf(os) >= 0) || "unknown";
};

export const isMacOs = (): boolean => {
  return getOs() === desktopOs.mac;
};