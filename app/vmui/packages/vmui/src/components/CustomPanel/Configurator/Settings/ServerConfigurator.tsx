import React, {FC, useState} from "preact/compat";
import TextField from "@mui/material/TextField";
import {useAppState} from "../../../../state/common/StateContext";
import {ErrorTypes} from "../../../../types";
import {ChangeEvent} from "react";

export interface ServerConfiguratorProps {
  error?: ErrorTypes | string;
  setServer: (url: string) => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({error, setServer}) => {

  const {serverUrl} = useAppState();
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  const onChangeServer = (e: ChangeEvent<HTMLTextAreaElement | HTMLInputElement>) => {
    const value = e.target.value || "";
    setChangedServerUrl(value);
    setServer(value);
  };

  return  <TextField variant="outlined" fullWidth label="Server URL" value={changedServerUrl || ""}
    error={error === ErrorTypes.validServer || error === ErrorTypes.emptyServer}
    inputProps={{style: {fontFamily: "Monospace"}}}
    onChange={onChangeServer}/>;
};

export default ServerConfigurator;
