import React, { FC, useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";

export interface ServerConfiguratorProps {
  serverUrl: string
  stateServerUrl: string
  onChange: (url: string) => void
  onEnter: () => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({
  serverUrl,
  stateServerUrl,
  onChange ,
  onEnter
}) => {

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

  return (
    <TextField
      autofocus
      label="Server URL"
      value={serverUrl}
      error={error}
      onChange={onChangeServer}
      onEnter={onEnter}
      inputmode="url"
    />
  );
};

export default ServerConfigurator;
