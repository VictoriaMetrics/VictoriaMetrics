import React, { FC, useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";
import Button from "../../../Main/Button/Button";
import { StorageIcon } from "../../../Main/Icons";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { getFromStorage, removeFromStorage, saveToStorage } from "../../../../utils/storage";
import useBoolean from "../../../../hooks/useBoolean";

export interface ServerConfiguratorProps {
  serverUrl: string
  stateServerUrl: string
  onChange: (url: string) => void
  onEnter: () => void
}

const tooltipSave = {
  enable: "Enable to save the modified server URL to local storage, preventing reset upon page refresh.",
  disable: "Disable to stop saving the server URL to local storage, reverting to the default URL on page refresh."
};

const ServerConfigurator: FC<ServerConfiguratorProps> = ({
  serverUrl,
  stateServerUrl,
  onChange ,
  onEnter
}) => {
  const {
    value: enabledStorage,
    toggle: handleToggleStorage,
  } = useBoolean(!!getFromStorage("SERVER_URL"));
  const [error, setError] = useState("");

  const onChangeServer = (val: string) => {
    const value = val || "";
    onChange(value);
    setError("");
  };

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
          onChange={onChangeServer}
          onEnter={onEnter}
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
};

export default ServerConfigurator;
