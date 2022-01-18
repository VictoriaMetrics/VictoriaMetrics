import React, {FC, useState} from "preact/compat";
import Tooltip from "@mui/material/Tooltip";
import SettingsIcon from "@mui/icons-material/Settings";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";
import Modal from "@mui/material/Modal";
import ServerConfigurator from "./ServerConfigurator";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import IconButton from "@mui/material/IconButton";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {getAppModeEnable} from "../../../../utils/app-mode";

const modalStyle = {
  position: "absolute" as const,
  top: "50%",
  left: "50%",
  transform: "translate(-50%, -50%)",
  bgcolor: "background.paper",
  p: 3,
  borderRadius: "4px",
  width: "80%",
  maxWidth: "800px"
};

const title = "Setting Server URL";

const GlobalSettings: FC = () => {

  const appModeEnable = getAppModeEnable();
  const {serverUrl} = useAppState();
  const dispatch = useAppDispatch();
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  const setServer = () => {
    if (!appModeEnable) dispatch({type: "SET_SERVER", payload: changedServerUrl});
    handleClose();
  };

  const [open, setOpen] = useState(false);
  const handleOpen = () => setOpen(true);
  const handleClose = () => setOpen(false);

  return <>
    <Tooltip title={title}>
      <Button variant="contained" color="primary"
        sx={{
          color: "white",
          border: "1px solid rgba(0, 0, 0, 0.2)",
          minWidth: "34px",
          padding: "6px 8px",
          boxShadow: "none",
        }}
        startIcon={<SettingsIcon style={{marginRight: "-8px", marginLeft: "4px"}}/>}
        onClick={handleOpen}>
      </Button>
    </Tooltip>
    <Modal open={open} onClose={handleClose}>
      <Box sx={modalStyle}>
        <Box display="grid" gridTemplateColumns="1fr auto" alignItems="center" mb={4}>
          <Typography id="modal-modal-title" variant="h6" component="h2">
            {title}
          </Typography>
          <IconButton size="small" onClick={handleClose}>
            <CloseIcon/>
          </IconButton>
        </Box>
        <ServerConfigurator setServer={setChangedServerUrl}/>
        <Box display="grid" gridTemplateColumns="auto auto" gap={1} justifyContent="end" mt={4}>
          <Button variant="outlined" onClick={handleClose}>
            Cancel
          </Button>
          <Button variant="contained" onClick={setServer}>
            apply
          </Button>
        </Box>
      </Box>
    </Modal>
  </>;
};

export default GlobalSettings;