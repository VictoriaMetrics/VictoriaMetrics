
export enum StorageErrorCode {
  NO_STORAGE = "NO_STORAGE",
  SECURITY_ERROR = "SECURITY_ERROR",
  QUOTA_EXCEEDED = "QUOTA_EXCEEDED",
  UNKNOWN = "UNKNOWN",
}

export type StorageError = {
  title: string;
  description: string;
  fix: string[]
}
