import classNames from "classnames";
import "./style.scss";
import { FC } from "preact/compat";
import { DoneIcon } from "../Icons";

interface CheckboxProps {
  checked: boolean
  color?: "primary" | "secondary" | "error"
  disabled?: boolean
  label?: string
  onChange: (value: boolean) => void
}

const Checkbox: FC<CheckboxProps> = ({
  checked = false, disabled = false, label, color = "secondary", onChange
}) => {
  const toggleCheckbox = () => {
    if (disabled) return;
    onChange(!checked);
  };

  const checkboxClasses = classNames({
    "vm-checkbox": true,
    "vm-checkbox_disabled": disabled,
    "vm-checkbox_active": checked,
    [`vm-checkbox_${color}_active`]: checked,
    [`vm-checkbox_${color}`]: color
  });

  return (
    <div
      className={checkboxClasses}
      onClick={toggleCheckbox}
    >
      <div className="vm-checkbox-track">
        <div className="vm-checkbox-track__thumb">
          <DoneIcon/>
        </div>
      </div>
      {label && <span className="vm-checkbox__label">{label}</span>}
    </div>
  );
};

export default Checkbox;
