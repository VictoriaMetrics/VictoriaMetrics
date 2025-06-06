import { FC } from "preact/compat";
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
  "data-id"?: string
  onClick?: (e: ReactMouseEvent<HTMLButtonElement>) => void
  onMouseDown?: (e: ReactMouseEvent<HTMLButtonElement>) => void
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
  "data-id": dataId
}) => {

  const classesButton = classNames({
    "vm-button": true,
    [`vm-button_${variant}_${color}`]: true,
    [`vm-button_${size}`]: size,
    "vm-button_icon_only": (startIcon || endIcon) && !children,
    "vm-button_full-width": fullWidth,
    "vm-button_with-icons": startIcon || endIcon,
    [className || ""]: className
  });

  return (
    <button
      className={classesButton}
      disabled={disabled}
      aria-label={ariaLabel}
      onClick={onClick}
      onMouseDown={onMouseDown}
      data-id={dataId}
    >
      {startIcon}{children}{endIcon}
    </button>
  );
};

export default Button;
