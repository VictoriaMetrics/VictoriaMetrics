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
    <Box display="grid" gridTemplateColumns="1fr auto" gap="4px" alignItems="center" width="100%" mb={2} minHeight={50}>
      <TextField variant="outlined" fullWidth label="Server URL" value={serverUrl}
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