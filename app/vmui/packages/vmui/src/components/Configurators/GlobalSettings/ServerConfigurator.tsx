import React, { FC, useState } from "preact/compat";
import TextField from "@mui/material/TextField";
import { useAppState } from "../../../state/common/StateContext";
import { ErrorTypes } from "../../../types";
import { ChangeEvent , KeyboardEvent } from "react";

export interface ServerConfiguratorProps {
  error?: ErrorTypes | string;
  setServer: (url: string) => void
  onEnter: (url: string) => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({ error, setServer , onEnter }) => {

  const { serverUrl } = useAppState();
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  const onChangeServer = (e: ChangeEvent<HTMLTextAreaElement | HTMLInputElement>) => {
    const value = e.target.value || "";
    setChangedServerUrl(value);
    setServer(value);
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      onEnter(changedServerUrl);
    }
  };
  return <TextField
    autoFocus
    fullWidth
    variant="outlined"
    label="Server URL"
    value={changedServerUrl || ""}
    error={error === ErrorTypes.validServer || error === ErrorTypes.emptyServer}
    inputProps={{ style: { fontFamily: "Monospace" } }}
    onChange={onChangeServer}
    onKeyDown={onKeyDown}
  />;
};

export default ServerConfigurator;
