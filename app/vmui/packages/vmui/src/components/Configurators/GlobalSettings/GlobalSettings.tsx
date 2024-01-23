import React, { FC, useEffect, useState } from "preact/compat";
import ServerConfigurator from "./ServerConfigurator/ServerConfigurator";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import { ArrowDownIcon, SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Modal from "../../Main/Modal/Modal";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";
import LimitsConfigurator from "./LimitsConfigurator/LimitsConfigurator";
import { Theme } from "../../../types";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { getAppModeEnable } from "../../../utils/app-mode";
import classNames from "classnames";
import Timezones from "./Timezones/Timezones";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import ThemeControl from "../ThemeControl/ThemeControl";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useBoolean from "../../../hooks/useBoolean";
import { getTenantIdFromUrl } from "../../../utils/tenants";
import { AppType } from "../../../types/appType";

const title = "Settings";

const { REACT_APP_TYPE } = process.env;
const isLogsApp = REACT_APP_TYPE === AppType.logs;

const GlobalSettings: FC = () => {
  const { isMobile } = useDeviceDetect();

  const appModeEnable = getAppModeEnable();
  const { serverUrl: stateServerUrl, theme } = useAppState();
  const { timezone: stateTimezone, defaultTimezone } = useTimeState();
  const { seriesLimits } = useCustomPanelState();

  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();
  const customPanelDispatch = useCustomPanelDispatch();

  const [serverUrl, setServerUrl] = useState(stateServerUrl);
  const [limits, setLimits] = useState(seriesLimits);
  const [timezone, setTimezone] = useState(stateTimezone);

  const setDefaultsValues = () => {
    setServerUrl(stateServerUrl);
    setLimits(seriesLimits);
    setTimezone(stateTimezone);
  };

  const {
    value: open,
    setTrue: handleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  const handleCloseAndReset = () => {
    handleClose();
    setDefaultsValues();
  };

  const handleChangeTheme = (value: Theme) => {
    dispatch({ type: "SET_THEME", payload: value });
  };

  const handlerApply = () => {
    const tenantIdFromUrl = getTenantIdFromUrl(serverUrl);
    if (tenantIdFromUrl !== "") {
      dispatch({ type: "SET_TENANT_ID", payload: tenantIdFromUrl });
    }
    dispatch({ type: "SET_SERVER", payload: serverUrl });
    timeDispatch({ type: "SET_TIMEZONE", payload: timezone });
    customPanelDispatch({ type: "SET_SERIES_LIMITS", payload: limits });
    handleClose();
  };

  useEffect(() => {
    // the tenant selector can change the serverUrl
    if (stateServerUrl === serverUrl) return;
    setServerUrl(stateServerUrl);
  }, [stateServerUrl]);

  useEffect(() => {
    setTimezone(stateTimezone);
  }, [stateTimezone]);

  const controls = [
    {
      show: !appModeEnable && !isLogsApp,
      component: <ServerConfigurator
        stateServerUrl={stateServerUrl}
        serverUrl={serverUrl}
        onChange={setServerUrl}
        onEnter={handlerApply}
      />
    },
    {
      show: !isLogsApp,
      component: <LimitsConfigurator
        limits={limits}
        onChange={setLimits}
        onEnter={handlerApply}
      />
    },
    {
      show: true,
      component: <Timezones
        timezoneState={timezone}
        defaultTimezone={defaultTimezone}
        onChange={setTimezone}
      />
    },
    {
      show: !appModeEnable,
      component: <ThemeControl
        theme={theme}
        onChange={handleChangeTheme}
      />
    }
  ].filter(control => control.show);

  return <>
    {isMobile ? (
      <div
        className="vm-mobile-option"
        onClick={handleOpen}
      >
        <span className="vm-mobile-option__icon"><SettingsIcon/></span>
        <div className="vm-mobile-option-text">
          <span className="vm-mobile-option-text__label">{title}</span>
        </div>
        <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
      </div>
    ) : (
      <Tooltip title={title}>
        <Button
          className={classNames({
            "vm-header-button": !appModeEnable
          })}
          variant="contained"
          color="primary"
          startIcon={<SettingsIcon/>}
          onClick={handleOpen}
          ariaLabel="settings"
        />
      </Tooltip>
    )}
    {open && (
      <Modal
        title={title}
        onClose={handleCloseAndReset}
      >
        <div
          className={classNames({
            "vm-server-configurator": true,
            "vm-server-configurator_mobile": isMobile
          })}
        >
          {controls.map((control, index) => (
            <div
              className="vm-server-configurator__input"
              key={index}
            >
              {control.component}
            </div>
          ))}
          <div className="vm-server-configurator-footer">
            <Button
              color="error"
              variant="outlined"
              onClick={handleCloseAndReset}
            >
              Cancel
            </Button>
            <Button
              color="primary"
              variant="contained"
              onClick={handlerApply}
            >
              Apply
            </Button>
          </div>
        </div>
      </Modal>
    )}
  </>;
};

export default GlobalSettings;
