import React, { CSSProperties, FC } from "preact/compat";
import "./style.scss";

interface SpinnerProps {
  containerStyles?: CSSProperties;
  message?: string
}

const Spinner: FC<SpinnerProps> = ({ containerStyles = {}, message }) => (
  <div
    className="vm-spinner"
    style={containerStyles && {}}
  >
    <div className="half-circle-spinner">
      <div className="circle circle-1"></div>
      <div className="circle circle-2"></div>
    </div>
    {message && <div className="vm-spinner__message">{message}</div>}
  </div>
);

export default Spinner;
