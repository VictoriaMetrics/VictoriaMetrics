export const isValidHttpUrl = (str: string): boolean => {
  let url;

  try {
    url = new URL(str);
  } catch (_) {
    return false;
  }

  return url.protocol === "http:" || url.protocol === "https:";
};

export const removeTrailingSlash = (url: string) => url.endsWith("/") ? url.slice(0, -1) : url;
