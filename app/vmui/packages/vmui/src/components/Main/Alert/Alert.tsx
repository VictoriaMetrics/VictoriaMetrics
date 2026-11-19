import { FC, ReactNode } from "preact/compat";
import classNames from "classnames";
import { DoneIcon, ErrorIcon, InfoIcon, WarningIcon } from "../Icons";
import "./style.scss";

interface AlertProps {
  variant?: "success" | "error" | "info" | "warning"
  children: ReactNode
  title?: string;
}

const icons = {
  success: <DoneIcon/>,
  error: <ErrorIcon/>,
  warning: <WarningIcon/>,
  info: <InfoIcon/>
};

const Alert: FC<AlertProps> = ({
  variant = "info",
  title,
  children
}) => {

  return (
    <div
      className={classNames({
        "vm-alert": true,
        [`vm-alert_${variant}`]: true
      })}
    >
      <div className="vm-alert__backdrop"/>
      <div className="vm-alert__icon">{icons[variant]}</div>
      {title && <div className="vm-alert__title">{title}</div>}
      {children && <div
        className={classNames({
          "vm-alert__content": title,
          "vm-alert__title": !title
        })}
      >{children}</div>}
    </div>
  );
};

export default Alert;
