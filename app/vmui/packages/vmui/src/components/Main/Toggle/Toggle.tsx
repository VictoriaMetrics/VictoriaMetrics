import { FC, useEffect, useRef, useState, ReactNode } from "preact/compat";
import classNames from "classnames";
import "./style.scss";

interface ToggleProps {
  options: { value: string, title?: string, icon?: ReactNode }[];
  value: string;
  onChange: (val: string) => void;
  label?: string;
  size?: "medium" | "large";
}

const Toggle: FC<ToggleProps> = ({ options, value, label, size = "medium", onChange }) => {

  const activeRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({
    width: "0px",
    left: "0px",
  });

  const createHandlerChange = (value: string) => () => {
    onChange(value);
  };

  useEffect(() => {
    if (!activeRef.current) {
      setPosition({
        width: "0px",
        left: "0px",
      });
      return;
    }
    const index = options.findIndex(o => o.value === value);
    const { width: widthRect } = activeRef.current.getBoundingClientRect();

    const width = widthRect;
    const left = index * width;

    setPosition({ width: `${width}px`, left: `${left}px` });
  }, [activeRef, value, options]);

  return (
    <div
      className={classNames({
        "vm-toggles": true,
        [`vm-toggles_${size}`]: size,
      })}
    >
      {label && (
        <label className="vm-toggles__label">
          {label}
        </label>
      )}
      <div
        className="vm-toggles-group"
        style={{ gridTemplateColumns: `repeat(${options.length}, 1fr)` }}
      >
        <div
          className="vm-toggles-group__highlight"
          style={position}
        />
        {options.map((option) => (
          <div
            className={classNames({
              "vm-toggles-group-item": true,
              "vm-toggles-group-item_active": option.value === value,
              "vm-toggles-group-item_icon": option.icon && option.title
            })}
            onClick={createHandlerChange(option.value)}
            key={option.value}
            ref={option.value === value ? activeRef : null}
          >
            {option.icon}
            {option.title}
          </div>
        ))}
      </div>
    </div>
  );
};

export default Toggle;
