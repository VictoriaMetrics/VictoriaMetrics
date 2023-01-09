import React, { FC, KeyboardEvent, useEffect, useRef, HTMLInputTypeAttribute, ReactNode } from "react";
import classNames from "classnames";
import { useMemo } from "preact/compat";
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
  onChange,
  onEnter,
  onKeyDown,
  onFocus,
  onBlur
}) => {

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
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onEnter && onEnter();
    }
  };

  const handleChange = (e: React.FormEvent) => {
    if (disabled) return;
    onChange && onChange((e.target as HTMLInputElement).value);
  };

  useEffect(() => {
    if (!autofocus) return;
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
          placeholder={placeholder}
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
          onInput={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={handleFocus}
          onBlur={handleBlur}
        />
      )
    }
    {label && <span className="vm-text-field__label">{label}</span>}
    <span
      className="vm-text-field__error"
      data-show={!!error}
    >
      {error}
    </span>
    {helperText && !error && (
      <span className="vm-text-field__helper-text">
        {helperText}
      </span>
    )}
  </label>;
};

export default TextField;
