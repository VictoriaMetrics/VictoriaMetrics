import React, { FC, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";

export interface ServerConfiguratorProps {
  serverUrl: string
  onChange: (url: string) => void
  onEnter: () => void
  onBlur: () => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({ serverUrl, onChange , onEnter, onBlur }) => {

  const [error, setError] = useState("");

  const onChangeServer = (val: string) => {
    const value = val || "";
    onChange(value);
    setError("");
    if (!value) setError(ErrorTypes.emptyServer);
    if (!isValidHttpUrl(value)) setError(ErrorTypes.validServer);
  };

  return (
    <TextField
      autofocus
      label="Server URL"
      value={serverUrl}
      error={error}
      onChange={onChangeServer}
      onEnter={onEnter}
      onBlur={onBlur}
      inputmode="url"
    />
  );
};

export default ServerConfigurator;
