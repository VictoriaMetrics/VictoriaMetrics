import React, { FC, KeyboardEvent, useEffect, useRef, HTMLInputTypeAttribute, ReactNode } from "react";
import classNames from "classnames";
import { useMemo } from "preact/compat";
import { useAppState } from "../../../state/common/StateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import TextFieldError from "./TextFieldError";
import "./style.scss";

interface TextFieldProps {
  label?: string,
  value?: string | number
  type?: HTMLInputTypeAttribute | "textarea"
  error?: string
  placeholder?: string
  endIcon?: ReactNode
  startIcon?: ReactNode
  disabled?: boolean
  autofocus?: boolean
  helperText?: string
  inputmode?: "search" | "text" | "email" | "tel" | "url" | "none" | "numeric" | "decimal"
  onChange?: (value: string) => void
  onEnter?: () => void
  onKeyDown?: (e: KeyboardEvent) => void
  onFocus?: () => void
  onBlur?: () => void
}

const TextField: FC<TextFieldProps> = ({
  label,
  value,
  type = "text",
  error = "",
  placeholder,
  endIcon,
  startIcon,
  disabled = false,
  autofocus = false,
  helperText,
  inputmode = "text",
  onChange,
  onEnter,
  onKeyDown,
  onFocus,
  onBlur
}) => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();

  const inputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fieldRef = useMemo(() => type === "textarea" ? textareaRef : inputRef, [type]);

  const inputClasses = classNames({
    "vm-text-field__input": true,
    "vm-text-field__input_error": error,
    "vm-text-field__input_icon-start": startIcon,
    "vm-text-field__input_disabled": disabled,
    "vm-text-field__input_textarea": type === "textarea",
  });

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    onKeyDown && onKeyDown(e);

    const { key, ctrlKey, metaKey } = e;
    const isEnter = key === "Enter";
    const runByEnter = type !== "textarea" ? isEnter : isEnter && (metaKey || ctrlKey);
    if (runByEnter && onEnter) {
      e.preventDefault();
      onEnter();
    }
  };

  const handleChange = (e: React.FormEvent) => {
    if (disabled) return;
    onChange && onChange((e.target as HTMLInputElement).value);
  };

  useEffect(() => {
    if (!autofocus || isMobile) return;
    fieldRef?.current?.focus && fieldRef.current.focus();
  }, [fieldRef, autofocus]);

  const handleFocus = () => {
    onFocus && onFocus();
  };

  const handleBlur = () => {
    onBlur && onBlur();
  };

  return <label
    className={classNames({
      "vm-text-field": true,
      "vm-text-field_textarea": type === "textarea",
      "vm-text-field_dark": isDarkTheme
    })}
    data-replicated-value={value}
  >
    {startIcon && <div className="vm-text-field__icon-start">{startIcon}</div>}
    {endIcon && <div className="vm-text-field__icon-end">{endIcon}</div>}
    {type === "textarea"
      ? (
        <textarea
          className={inputClasses}
          disabled={disabled}
          ref={textareaRef}
          value={value}
          rows={1}
          inputMode={inputmode}
          placeholder={placeholder}
          autoCapitalize={"none"}
          onInput={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={handleFocus}
          onBlur={handleBlur}
        />
      )
      : (
        <input
          className={inputClasses}
          disabled={disabled}
          ref={inputRef}
          value={value}
          type={type}
          placeholder={placeholder}
          inputMode={inputmode}
          autoCapitalize={"none"}
          onInput={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={handleFocus}
          onBlur={handleBlur}
        />
      )
    }
    {label && <span className="vm-text-field__label">{label}</span>}
    <TextFieldError error={error}/>
    {helperText && !error && (
      <span className="vm-text-field__helper-text">
        {helperText}
      </span>
    )}
  </label>
  ;
};

export default TextField;
