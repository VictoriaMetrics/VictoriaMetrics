import React, { FC } from "preact/compat";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent, ReactNode } from "react";
import "./style.scss";

interface ButtonProps {
  variant?: "contained" | "outlined" | "text"
  color?: "primary" | "secondary" | "success" | "error" | "gray"  | "warning"
  size?: "small" | "medium" | "large"
  endIcon?: ReactNode
  startIcon?: ReactNode
  fullWidth?: boolean
  disabled?: boolean
  children?: ReactNode
  className?: string
  onClick?: (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => void
}

const Button: FC<ButtonProps> = ({
  variant = "contained",
  color = "primary",
  size = "medium",
  children,
  endIcon,
  startIcon,
  fullWidth = false,
  className,
  disabled,
  onClick,
}) => {

  const classesButton = classNames({
    "vm-button": true,
    [`vm-button_${variant}_${color}`]: true,
    [`vm-button_${size}`]: size,
    "vm-button_icon": (startIcon || endIcon) && !children,
    "vm-button_full-width": fullWidth,
    "vm-button_with-icon": startIcon || endIcon,
    "vm-button_disabled": disabled,
    [className || ""]: className
  });

  return (
    <button
      className={classesButton}
      disabled={disabled}
      onClick={onClick}
    >
      <>
        {startIcon && <span className="vm-button__start-icon">{startIcon}</span>}
        {children && <span>{children}</span>}
        {endIcon && <span className="vm-button__end-icon">{endIcon}</span>}
      </>
    </button>
  );
};

export default Button;
