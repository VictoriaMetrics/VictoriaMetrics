import React, { ReactNode } from "react";
import classNames from "classnames";
import "./style.scss";
import { FC } from "preact/compat";

interface SwitchProps {
  value: boolean
  color?: "primary" | "secondary" | "error"
  disabled?: boolean
  label?: string | ReactNode
  onChange: (value: boolean) => void
}

const Switch: FC<SwitchProps> = ({
  value = false, disabled = false, label, color = "secondary", onChange
}) => {
  const toggleSwitch = () => {
    if (disabled) return;
    onChange(!value);
  };

  const switchClasses = classNames({
    "vm-switch": true,
    "vm-switch_disabled": disabled,
    "vm-switch_active": value,
    [`vm-switch_${color}_active`]: value,
    [`vm-switch_${color}`]: color
  });

  return (
    <div
      className={switchClasses}
      onClick={toggleSwitch}
    >
      <div className="vm-switch-track">
        <div
          className="vm-switch-track__thumb"
        />
      </div>
      {label && <span className="vm-switch__label">{label}</span>}
    </div>
  );
};

export default Switch;
