import { FC, useEffect, useState } from "preact/compat";
import Alert from "../../../Main/Alert/Alert";
import Hyperlink from "../../../Main/Hyperlink/Hyperlink";
import { getFromStorage } from "../../../../utils/storage";
import { useAppState } from "../../../../state/common/StateContext";
import { getDefaultURL } from "../../../../utils/default-server-url";
import { replaceTenantId } from "../../../../utils/tenants";
import { removeTrailingSlash } from "../../../../utils/url";
import "../style.scss";

const normalizeServerUrl = (url: string) => {
  try {
    const normalizedUrl = new URL(url).href;
    return replaceTenantId(removeTrailingSlash(normalizedUrl), "0");
  } catch {
    return replaceTenantId(removeTrailingSlash(url), "0");
  }
};

const isServerUrlSaved = () => {
  try {
    return !!getFromStorage("SERVER_URL");
  } catch {
    return false;
  }
};

type Props = {
  onOpenSettings?: () => void;
}

const ServerUrlDeprecationWarning: FC<Props> = ({ onOpenSettings }) => {
  const { serverUrl } = useAppState();
  const [enabledStorage, setEnabledStorage] = useState(isServerUrlSaved);
  const defaultServerUrl = getDefaultURL(window.location.href);
  const isCustomServerUrl = normalizeServerUrl(serverUrl) !== normalizeServerUrl(defaultServerUrl);

  useEffect(() => {
    const updateEnabledStorage = () => setEnabledStorage(isServerUrlSaved());
    updateEnabledStorage();
    window.addEventListener("storage", updateEnabledStorage);
    return () => window.removeEventListener("storage", updateEnabledStorage);
  }, []);

  if (!enabledStorage || !isCustomServerUrl) return null;

  return (
    <div className="vm-server-url-deprecation-warning">
      <Alert variant="warning">
        <p>
          You are using a custom Server URL. This option will be removed in a future release.{" "}
          <button
            type="button"
            className="vm-link vm-link_colored"
            onClick={onOpenSettings}
          >
            Open settings
          </button>{" "}
          to review your URL, or{" "}
          <Hyperlink
            href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11735"
            text={onOpenSettings ? "share your use case in #11735." : "Share your use case in #11735."}
          />
        </p>
      </Alert>
    </div>
  );
};

export default ServerUrlDeprecationWarning;
