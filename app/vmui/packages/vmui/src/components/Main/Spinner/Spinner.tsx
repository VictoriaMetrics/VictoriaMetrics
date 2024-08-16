import React, { FC } from "preact/compat";
import { CSSProperties } from "react";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../../state/common/StateContext";

interface SpinnerProps {
  containerStyles?: CSSProperties;
  message?: string
}

const Spinner: FC<SpinnerProps> = ({ containerStyles, message }) => {
  const { isDarkTheme } = useAppState();

  return (
    <div
      className={classNames({
        "vm-spinner": true,
        "vm-spinner_dark": isDarkTheme,
      })}
      style={containerStyles}
    >
      <div className="half-circle-spinner">
        <div className="circle circle-1"></div>
        <div className="circle circle-2"></div>
      </div>
      {message && <div className="vm-spinner__message">{message}</div>}
    </div>
  );
};

export default Spinner;
