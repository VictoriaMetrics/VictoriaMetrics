import { forwardRef, MouseEvent as ReactMouseEvent, ReactNode } from "react";
import classNames from "classnames";
import "./style.scss";

interface ButtonProps {
  variant?: "contained" | "outlined" | "text";
  color?: "primary" | "secondary" | "success" | "error" | "gray" | "warning" | "white";
  size?: "small" | "medium" | "large";
  ariaLabel?: string; // https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Attributes/aria-label
  endIcon?: ReactNode;
  startIcon?: ReactNode;
  fullWidth?: boolean;
  disabled?: boolean;
  children?: ReactNode;
  className?: string;
  onClick?: (e: ReactMouseEvent<HTMLButtonElement>) => void;
  onMouseDown?: (e: ReactMouseEvent<HTMLButtonElement>) => void;
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>((props, ref) => {
  const {
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
  } = props;

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
      ref={ref}
      className={classesButton}
      disabled={disabled}
      aria-label={ariaLabel}
      onClick={onClick}
      onMouseDown={onMouseDown}
    >
      {startIcon}{children}{endIcon}
    </button>
  );
});

Button.displayName = "Button";

export default Button;
