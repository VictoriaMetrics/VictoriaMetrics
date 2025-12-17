import { useEffect } from "react";
import { StorageErrorCode } from "./types";
import { useSnack } from "../../contexts/Snackbar";
import { storageErrorInfo } from "./storageErrors";
import "./style.scss";

const classifyStorageException = (e: unknown): StorageErrorCode => {
  if (!(e instanceof DOMException)) return StorageErrorCode.UNKNOWN;

  switch (e.name) {
    case "QuotaExceededError":
      return StorageErrorCode.QUOTA_EXCEEDED;
    case "SecurityError":
      return StorageErrorCode.SECURITY_ERROR;
    default:
      return StorageErrorCode.UNKNOWN;
  }
};

const getStorageError = (storage: Storage | null | undefined): StorageErrorCode | null => {
  if (!storage) {
    return StorageErrorCode.NO_STORAGE;
  }

  try {
    const key = "__vmui_test__";
    storage.setItem(key, "1");
    storage.removeItem(key);
    return null;
  } catch (e) {
    return classifyStorageException(e);
  }
};

const WebStorageCheck = () => {
  const { showInfoMessage } = useSnack();

  useEffect(() => {
    const error = getStorageError(window.localStorage);

    if (error) {
      const { title, description, fix } = storageErrorInfo[error];

      const text = (
        <div className="vm-storage-check">
          <h3>{title}</h3>
          <p>{description}</p>

          {!!fix?.length && (
            <div className="vm-storage-check__fix">
              <div>Try this:</div>
              <ul>
                {fix.map((step, i) => (
                  <li key={`${i}-${step}`}>{step}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      );

      showInfoMessage({ text: text, type: "error", timeout: 600000 });
    }
  }, []);

  return null;
};

export default WebStorageCheck;
