import React, { FC, useState } from "preact/compat";
import ServerConfigurator from "./ServerConfigurator/ServerConfigurator";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Modal from "../../Main/Modal/Modal";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";
import LimitsConfigurator from "./LimitsConfigurator/LimitsConfigurator";
import { SeriesLimits } from "../../../types";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";

const title = "Settings";

const GlobalSettings: FC = () => {

  const { serverUrl: stateServerUrl } = useAppState();
  const { seriesLimits } = useCustomPanelState();

  const dispatch = useAppDispatch();
  const customPanelDispatch = useCustomPanelDispatch();

  const [serverUrl, setServerUrl] = useState(stateServerUrl);
  const [limits, setLimits] = useState<SeriesLimits>(seriesLimits);

  const [open, setOpen] = useState(false);
  const handleOpen = () => setOpen(true);
  const handleClose = () => setOpen(false);

  const handlerApply = () => {
    dispatch({ type: "SET_SERVER", payload: serverUrl });
    customPanelDispatch({ type: "SET_SERIES_LIMITS", payload: limits });
    handleClose();
  };

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
              serverUrl={serverUrl}
              onChange={setServerUrl}
              onEnter={handlerApply}
            />
          </div>
          <div className="vm-server-configurator__input">
            <LimitsConfigurator
              limits={limits}
              onChange={setLimits}
              onEnter={handlerApply}
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
              onClick={handlerApply}
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
