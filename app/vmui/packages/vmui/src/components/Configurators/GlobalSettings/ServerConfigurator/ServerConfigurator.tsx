import React, { FC, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";

export interface ServerConfiguratorProps {
  serverUrl: string
  onChange: (url: string) => void
  onEnter: () => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({ serverUrl, onChange , onEnter }) => {

  const [error, setError] = useState("");

  const onChangeServer = (val: string) => {
    const value = val || "";
    onChange(value);
    setError("");
    if (!value) setError(ErrorTypes.emptyServer);
    if (!isValidHttpUrl(value)) setError(ErrorTypes.validServer);
  };

  return (
    <div>
      <div className="vm-server-configurator__title">
        Server URL
      </div>
      <TextField
        autofocus
        value={serverUrl}
        error={error}
        onChange={onChangeServer}
        onEnter={onEnter}
      />
    </div>
  );
};

export default ServerConfigurator;
