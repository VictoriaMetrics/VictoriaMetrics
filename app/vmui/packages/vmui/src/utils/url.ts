export const isValidHttpUrl = (str: string): boolean => {
  let url;

  try {
    url = new URL(str);
  } catch (_) {
    return false;
  }

  return url.protocol === "http:" || url.protocol === "https:";
};

export const removeTrailingSlash = (url: string) => url.replace(/\/$/, "");

export const isEqualURLSearchParams = (params1: URLSearchParams, params2: URLSearchParams): boolean => {
  if (Array.from(params1.entries()).length !== Array.from(params2.entries()).length) {
    return false;
  }

  for (const [key, value] of params1) {
    if (params2.get(key) !== value) {
      return false;
    }
  }

  return true;
};

export const getApiEndpoint = (url: string): string | null => {
  try {
    const match = url.match(/\/api\/v1\/[^?]+/);
    return match ? match[0] : null;
  } catch (error) {
    console.error("Invalid URL:", error);
    return null;
  }
};
