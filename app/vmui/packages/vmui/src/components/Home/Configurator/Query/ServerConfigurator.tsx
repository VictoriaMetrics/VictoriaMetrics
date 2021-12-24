import React, {FC, useEffect, useState} from "preact/compat";
import {Box, TextField, Tooltip, IconButton} from "@mui/material";
import SecurityIcon from "@mui/icons-material/Security";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {AuthDialog} from "../Auth/AuthDialog";
import {ErrorTypes} from "../../../../types";
import {getAppModeEnable, getAppModeParams} from "../../../../utils/app-mode";

export interface ServerConfiguratorProps {
  error?: ErrorTypes | string;
}

const ServerConfigurator: FC<ServerConfiguratorProps> = ({error}) => {

  const appModeEnable = getAppModeEnable();
  const {serverURL: appServerUrl} = getAppModeParams();

  const {serverUrl} = useAppState();
  const dispatch = useAppDispatch();

  const onSetServer = ({target: {value}}: {target: {value: string}}) => {
    dispatch({type: "SET_SERVER", payload: value});
  };
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    if (appModeEnable) dispatch({type: "SET_SERVER", payload: appServerUrl});
  }, [appServerUrl]);

  return <>
    <Box display="grid" gridTemplateColumns="1fr auto" gap="4px" alignItems="center" width="100%" mb={2} minHeight={50}>
      <TextField variant="outlined" fullWidth label="Server URL" value={serverUrl || ""} disabled={appModeEnable}
        error={error === ErrorTypes.validServer || error === ErrorTypes.emptyServer}
        inputProps={{style: {fontFamily: "Monospace"}}}
        onChange={onSetServer}/>
      <Box>
        <Tooltip title="Request Auth Settings">
          <IconButton onClick={() => setDialogOpen(true)}>
            <SecurityIcon/>
          </IconButton>
        </Tooltip>
      </Box>
    </Box>
    <AuthDialog open={dialogOpen} onClose={() => setDialogOpen(false)}/>
  </>;
};

export default ServerConfigurator;