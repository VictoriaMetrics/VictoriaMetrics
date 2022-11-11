import React, { CSSProperties, FC } from "preact/compat";
import "./style.scss";

interface SpinnerProps {
  containerStyles?: CSSProperties;
}

const Spinner: FC<SpinnerProps> = ({ containerStyles = {} }) => (
  <div
    className="vm-spinner"
    style={containerStyles && {}}
  >
    <div className="half-circle-spinner">
      <div className="circle circle-1"></div>
      <div className="circle circle-2"></div>
    </div>
  </div>
);

export default Spinner;
