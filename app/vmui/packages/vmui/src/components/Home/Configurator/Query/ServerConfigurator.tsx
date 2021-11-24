import React, {FC, useState} from "react";
import {Box, TextField, Tooltip, IconButton} from "@mui/material";
import SecurityIcon from "@mui/icons-material/Security";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {AuthDialog} from "../Auth/AuthDialog";


const ServerConfigurator: FC = () => {

  const {serverUrl} = useAppState();
  const dispatch = useAppDispatch();

  const onSetServer = ({target: {value}}: {target: {value: string}}) => {
    dispatch({type: "SET_SERVER", payload: value});
  };
  const [dialogOpen, setDialogOpen] = useState(false);

  return <>
    <Box display="flex" alignItems="center" mb={2} minHeight={50}>
      <TextField variant="outlined" fullWidth label="Server URL" value={serverUrl}
        inputProps={{style: {fontFamily: "Monospace"}}}
        onChange={onSetServer}/>
      <Box>
        <Tooltip title="Request Auth Settings">
          <IconButton onClick={() => setDialogOpen(true)} size="large">
            <SecurityIcon/>
          </IconButton>
        </Tooltip>
      </Box>
    </Box>
    <AuthDialog open={dialogOpen} onClose={() => setDialogOpen(false)}/>
  </>;
};

export default ServerConfigurator;