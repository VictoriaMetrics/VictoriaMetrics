import faviconRaw from "../../assets/favicon.svg?raw";

export const createFaviconUrl = (color = "#020202"): string => {
  const svgDocument = new DOMParser().parseFromString(faviconRaw, "image/svg+xml");
  const svg = svgDocument.documentElement;

  if (svg.localName !== "svg") {
    throw new Error("Invalid favicon SVG");
  }

  svg.setAttribute("fill", color);

  const serializedSvg = new XMLSerializer().serializeToString(svg);
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(serializedSvg)}`;
};

export const updateFaviconColor = (color = "#020202"): void => {
  const favicon = document.querySelector<HTMLLinkElement>("#favicon");
  if (favicon) {
    favicon.href = createFaviconUrl(color);
  }

  const maskIcon = document.querySelector<HTMLLinkElement>("#mask-icon");
  if (maskIcon) {
    maskIcon.setAttribute("color", color);
  }
};

export const getFaviconStorageKey = () => window.location.pathname.replace(/\/+$/, "") || "/";
