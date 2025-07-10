import { forwardRef, useCallback, useEffect, useImperativeHandle, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";
import Button from "../../../Main/Button/Button";
import { StorageIcon } from "../../../Main/Icons";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { getFromStorage, removeFromStorage, saveToStorage } from "../../../../utils/storage";
import useBoolean from "../../../../hooks/useBoolean";
import { ChildComponentHandle } from "../GlobalSettings";
import { useAppDispatch, useAppState } from "../../../../state/common/StateContext";
import { getTenantIdFromUrl } from "../../../../utils/tenants";

interface ServerConfiguratorProps {
  onClose: () => void;
}

const tooltipSave = {
  enable: "Enable to save the modified server URL to local storage, preventing reset upon page refresh.",
  disable: "Disable to stop saving the server URL to local storage, reverting to the default URL on page refresh."
};

const ServerConfigurator = forwardRef<ChildComponentHandle, ServerConfiguratorProps>(({ onClose }, ref) => {
  const { serverUrl: stateServerUrl } = useAppState();
  const dispatch = useAppDispatch();

  const {
    value: enabledStorage,
    toggle: handleToggleStorage,
  } = useBoolean(!!getFromStorage("SERVER_URL"));

  const [serverUrl, setServerUrl] = useState(stateServerUrl);
  const [error, setError] = useState("");

  const handleChange = (val: string) => {
    const value = val || "";
    setServerUrl(value);
    setError("");
  };

  const handleApply = useCallback(() => {
    const tenantIdFromUrl = getTenantIdFromUrl(serverUrl);
    if (tenantIdFromUrl !== "") {
      dispatch({ type: "SET_TENANT_ID", payload: tenantIdFromUrl });
    }
    dispatch({ type: "SET_SERVER", payload: serverUrl });
    onClose();
  }, [serverUrl]);

  useEffect(() => {
    if (!stateServerUrl) setError(ErrorTypes.emptyServer);
    if (!isValidHttpUrl(stateServerUrl)) setError(ErrorTypes.validServer);
  }, [stateServerUrl]);

  useEffect(() => {
    if (enabledStorage) {
      saveToStorage("SERVER_URL", serverUrl);
    } else {
      removeFromStorage(["SERVER_URL"]);
    }
  }, [enabledStorage]);

  useEffect(() => {
    if (enabledStorage) {
      saveToStorage("SERVER_URL", serverUrl);
    }
  }, [serverUrl]);

  useEffect(() => {
    // the tenant selector can change the serverUrl
    if (stateServerUrl === serverUrl) return;
    setServerUrl(stateServerUrl);
  }, [stateServerUrl]);

  useImperativeHandle(ref, () => ({ handleApply }), [handleApply]);

  return (
    <div>
      <div className="vm-server-configurator__title">
        Server URL
      </div>
      <div className="vm-server-configurator-url">
        <TextField
          autofocus
          value={serverUrl}
          error={error}
          onChange={handleChange}
          onEnter={handleApply}
          inputmode="url"
        />
        <Tooltip title={enabledStorage ? tooltipSave.disable : tooltipSave.enable}>
          <Button
            className="vm-server-configurator-url__button"
            variant="text"
            color={enabledStorage ? "primary" : "gray"}
            onClick={handleToggleStorage}
            startIcon={<StorageIcon/>}
          />
        </Tooltip>
      </div>
    </div>
  );
});

export default ServerConfigurator;
