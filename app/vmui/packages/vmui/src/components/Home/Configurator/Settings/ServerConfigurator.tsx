import React, {FC, useEffect, useState} from "preact/compat";
import TextField from "@mui/material/TextField";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {ErrorTypes} from "../../../../types";
import {getAppModeEnable, getAppModeParams} from "../../../../utils/app-mode";
import {ChangeEvent} from "react";

export interface ServerConfiguratorProps {
  error?: ErrorTypes | string;
  setServer: (url: string) => void
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({error, setServer}) => {

  const appModeEnable = getAppModeEnable();
  const {serverURL: appServerUrl} = getAppModeParams();

  const {serverUrl} = useAppState();
  const dispatch = useAppDispatch();
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  useEffect(() => {
    if (appModeEnable) {
      dispatch({type: "SET_SERVER", payload: appServerUrl});
      setChangedServerUrl(appServerUrl);
    }
  }, [appServerUrl]);

  const onChangeServer = (e: ChangeEvent<HTMLTextAreaElement | HTMLInputElement>) => {
    const value = e.target.value || "";
    setChangedServerUrl(value);
    setServer(value);
  };

  return  <TextField variant="outlined" fullWidth label="Server URL" value={changedServerUrl || ""} disabled={appModeEnable}
    error={error === ErrorTypes.validServer || error === ErrorTypes.emptyServer}
    inputProps={{style: {fontFamily: "Monospace"}}}
    onChange={onChangeServer}/>;
};

export default ServerConfigurator;