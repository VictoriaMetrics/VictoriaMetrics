import React, { FC, useState } from "preact/compat";
import { useAppState } from "../../../../state/common/StateContext";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";

export interface ServerConfiguratorProps {
  setServer: (url: string) => void
  onEnter: (url: string) => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({ setServer , onEnter }) => {

  const { serverUrl } = useAppState();
  const [error, setError] = useState("");
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  const onChangeServer = (val: string) => {
    const value = val || "";
    setChangedServerUrl(value);
    setServer(value);
    setError("");
    if (!value) setError(ErrorTypes.emptyServer);
    if (!isValidHttpUrl(value)) setError(ErrorTypes.validServer);
  };

  return (
    <TextField
      autofocus
      label="Server URL"
      value={changedServerUrl || ""}
      error={error}
      onChange={onChangeServer}
      onEnter={() => onEnter(changedServerUrl)}
    />
  );
};

export default ServerConfigurator;
