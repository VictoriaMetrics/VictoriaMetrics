import React, { FC, useState } from "preact/compat";
import ServerConfigurator from "./ServerConfigurator/ServerConfigurator";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Modal from "../../Main/Modal/Modal";

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
    {/*<Tooltip title={title}>*/}
    <Button
      variant="contained"
      color="primary"
      // sx={{
      //   color: "white",
      //   border: "1px solid rgba(0, 0, 0, 0.2)",
      //   minWidth: "34px",
      //   padding: "6px 8px",
      //   boxShadow: "none",
      // }}
      onClick={handleOpen}
    >
      <SettingsIcon/>
    </Button>
    {/*</Tooltip>*/}
    {open && (
      <Modal
        title={title}
        onClose={handleClose}
      >
        <div>
          <div>
            <ServerConfigurator
              setServer={setChangedServerUrl}
              onEnter={setServer}
            />
          </div>

          {/*
          // TODO modal footer
          display="grid"
          gridTemplateColumns="auto auto"
          gap={1}
          justifyContent="end"
          mt={4}
          */}
          <div>
            <Button
              variant="outlined"
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
