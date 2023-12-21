import React, { FC } from "preact/compat";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent, ReactNode } from "react";
import "./style.scss";

interface ButtonProps {
  variant?: "contained" | "outlined" | "text"
  color?: "primary" | "secondary" | "success" | "error" | "gray"  | "warning" | "white"
  size?: "small" | "medium" | "large"
  ariaLabel?: string // https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-label
  endIcon?: ReactNode
  startIcon?: ReactNode
  fullWidth?: boolean
  disabled?: boolean
  children?: ReactNode
  className?: string
  onClick?: (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => void
  onMouseDown?: (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => void
}

const Button: FC<ButtonProps> = ({
  variant = "contained",
  color = "primary",
  size = "medium",
  ariaLabel,
  children,
  endIcon,
  startIcon,
  fullWidth = false,
  className,
  disabled,
  onClick,
  onMouseDown,
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
      aria-label={ariaLabel}
      onClick={onClick}
      onMouseDown={onMouseDown}
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
