const desktopOs = ["Windows", "Mac", "Linux"];

export const getOs = () : string => {
  return desktopOs.find(os => navigator.userAgent.indexOf(os) >= 0) || "unknown";
};

export const isMacOs = (): boolean => {
  return getOs() === "Mac";
};