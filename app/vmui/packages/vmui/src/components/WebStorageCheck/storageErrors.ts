import { StorageError, StorageErrorCode } from "./types";

export const storageErrorInfo: Record<StorageErrorCode, StorageError> = {
  [StorageErrorCode.NO_STORAGE]: {
    title: "Storage unavailable",
    description:
      "Browser storage is not available for this website.",
    fix: [
      "Disable Private/Incognito mode and reload the page.",
      "Disable privacy or ad-blocking extensions for this site and reload.",
      "Open the site in another browser.",
    ],
  },

  [StorageErrorCode.SECURITY_ERROR]: {
    title: "Storage access blocked",
    description:
      "Browser settings or an extension are blocking access to browser storage.",
    fix: [
      "Disable Private/Incognito mode and reload the page.",
      "Disable privacy or ad-blocking extensions for this site and reload.",
      "Open the site in a regular browser tab (not embedded).",
    ],
  },

  [StorageErrorCode.QUOTA_EXCEEDED]: {
    title: "Storage quota exceeded",
    description:
      "The storage limit for this website has been reached.",
    fix: [
      "Clear this website’s stored data and reload the page.",
      "Close other tabs for this website and try again.",
      "Use another browser or browser profile.",
    ],
  },

  [StorageErrorCode.UNKNOWN]: {
    title: "Storage error",
    description:
      "An unexpected error occurred while accessing browser storage.",
    fix: [
      "Reload the page.",
      "Update the browser and try again.",
      "Disable browser extensions and reload.",
    ],
  },
};
