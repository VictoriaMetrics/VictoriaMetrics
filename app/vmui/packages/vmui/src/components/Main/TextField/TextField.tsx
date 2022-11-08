import React, { FC, KeyboardEvent, useEffect, useRef, HTMLInputTypeAttribute, ReactNode, LegacyRef } from "react";
import classNames from "classnames";
import "./style.scss";
import { useMemo } from "preact/compat";

interface TextFieldProps {
  label?: string,
  value?: string | number
  type?: HTMLInputTypeAttribute | "textarea"
  error?: string
  endIcon?: ReactNode
  startIcon?: ReactNode
  disabled?: boolean
  autofocus?: boolean
  onChange?: (value: string) => void
  onEnter?: () => void
  onKeyDown?: (e: KeyboardEvent) => void
}

const TextField: FC<TextFieldProps> = ({
  label,
  value,
  type = "text",
  error = "",
  endIcon,
  startIcon,
  disabled = false,
  autofocus = false,
  onChange,
  onEnter,
  onKeyDown
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
    if (e.key === "Enter" && onEnter) {
      e.preventDefault();
      onEnter();
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

  return <label className="vm-text-field">
    {startIcon && <div className="vm-text-field__icon-start">{startIcon}</div>}
    {endIcon && <div className="vm-text-field__icon-end">{endIcon}</div>}
    {type === "textarea"
      ? (
        <textarea
          className={inputClasses}
          disabled={disabled}
          ref={textareaRef}
          value={value}
          onInput={handleChange}
          onKeyDown={handleKeyDown}
          placeholder=" "
        />
      )
      : (
        <input
          className={inputClasses}
          disabled={disabled}
          ref={inputRef}
          value={value}
          onInput={handleChange}
          onKeyDown={handleKeyDown}
          placeholder=" "
          type={type}
        />)
    }
    {label && <span className="vm-text-field__label">{label}</span>}
    <span
      className="vm-text-field__error"
      data-show={!!error}
    >
      {error}
    </span>
  </label>;
};

export default TextField;
