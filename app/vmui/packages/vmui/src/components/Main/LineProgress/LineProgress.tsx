import React from "preact/compat";
import "./style.scss";

const LineProgress = ({ value }: {value: number}) => (
  <div className="vm-line-progress">
    <div className="vm-line-progress-track">
      <div
        className="vm-line-progress-track__thumb"
        style={{ width: `${value}%` }}
      />
    </div>
    <span>{value.toFixed(2)}%</span>
  </div>
);

export default LineProgress;
