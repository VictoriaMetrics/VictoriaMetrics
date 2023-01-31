import React, { FC } from "preact/compat";
import { ReactNode } from "react";
import classNames from "classnames";
import { ErrorIcon, InfoIcon, SuccessIcon, WarningIcon } from "../Icons";
import "./style.scss";
import { useAppState } from "../../../state/common/StateContext";

interface AlertProps {
  variant?: "success" | "error" | "info" | "warning"
  children: ReactNode
}

const icons = {
  success: <SuccessIcon/>,
  error: <ErrorIcon/>,
  warning: <WarningIcon/>,
  info: <InfoIcon/>
};

const Alert: FC<AlertProps> = ({
  variant,
  children }) => {
  const { isDarkTheme } = useAppState();

  return (
    <div
      className={classNames({
        "vm-alert": true,
        [`vm-alert_${variant}`]: variant,
        "vm-alert_dark": isDarkTheme
      })}
    >
      <div className="vm-alert__icon">{icons[variant || "info"]}</div>
      <div className="vm-alert__content">{children}</div>
    </div>
  );
};

export default Alert;
