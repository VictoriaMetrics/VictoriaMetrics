import React, { FC, useState } from "preact/compat";
import ServerConfigurator from "./ServerConfigurator/ServerConfigurator";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Modal from "../../Main/Modal/Modal";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";

const title = "Setting Server URL";

const GlobalSettings: FC = () => {

  const { serverUrl } = useAppState();
  const dispatch = useAppDispatch();
  const [changedServerUrl, setChangedServerUrl] = useState(serverUrl);

  const setServer = (url?: string) => {
    dispatch({ type: "SET_SERVER", payload: url || changedServerUrl });
    handleClose();
  };

  const [open, setOpen] = useState(false);
  const handleOpen = () => setOpen(true);
  const handleClose = () => setOpen(false);

  return <>
    <Tooltip title={title}>
      <Button
        className="vm-header-button"
        variant="contained"
        color="primary"
        startIcon={<SettingsIcon/>}
        onClick={handleOpen}
      />
    </Tooltip>
    {open && (
      <Modal
        title={title}
        onClose={handleClose}
      >
        <div className="vm-server-configurator">
          <div className="vm-server-configurator__input">
            <ServerConfigurator
              setServer={setChangedServerUrl}
              onEnter={setServer}
            />
          </div>
          <div className="vm-server-configurator__footer">
            <Button
              variant="outlined"
              color="error"
              onClick={handleClose}
            >
                Cancel
            </Button>
            <Button
              variant="contained"
              onClick={() => setServer()}
            >
                apply
            </Button>
          </div>
        </div>
      </Modal>
    )}
  </>;
};

export default GlobalSettings;
