import React from "react";
import classNames from "classnames";
import "./style.scss";
import { FC } from "preact/compat";

interface SwitchProps {
  value: boolean
  disabled?: boolean
  label?: string
  onChange: (value: boolean) => void
}

const Switch: FC<SwitchProps> = ({ value = false, disabled = false, label, onChange }) => {
  const toggleSwitch = () => {
    if (disabled) return;
    onChange(!value);
  };

  const switchClasses = classNames({
    "vm-switch": true,
    "vm-switch_disabled": disabled,
    "vm-switch_active": value,
  });

  return (
    <div
      className={switchClasses}
      onClick={toggleSwitch}
    >
      <div
        className="vm-switch__handle"
      />
      {label && <span className="vm-switch__label">{label}</span>}
    </div>
  );
};

export default Switch;
