const desktopOs = {
  windows: "Windows",
  mac: "Mac OS",
  linux: "Linux"
};

export const getOs = () => {
  return Object.values(desktopOs).find(os => navigator.userAgent.indexOf(os) >= 0) || "unknown";
};

export const isMacOs = () => {
  return getOs() === desktopOs.mac;
};

export const isMobileAgent = () => {
  const mobileUserAgents = [
    "Android",
    "webOS",
    "iPhone",
    "iPad",
    "iPod",
    "BlackBerry",
    "Windows Phone",
  ];

  // check for common mobile user agents
  const matches = mobileUserAgents.map(m => navigator.userAgent.match(new RegExp(m, "i")));
  return matches.some(m => m);
};
