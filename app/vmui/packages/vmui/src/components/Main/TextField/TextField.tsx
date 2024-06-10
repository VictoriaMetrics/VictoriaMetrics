import React, {
  FC,
  useEffect,
  useState,
  useRef,
  useMemo,
  FormEvent,
  KeyboardEvent,
  MouseEvent,
  HTMLInputTypeAttribute,
  ReactNode
} from "react";
import classNames from "classnames";
import { useAppState } from "../../../state/common/StateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import TextFieldMessage from "./TextFieldMessage";
import "./style.scss";

interface TextFieldProps {
  label?: string,
  value?: string | number
  type?: HTMLInputTypeAttribute | "textarea"
  error?: string
  warning?: string
  placeholder?: string
  endIcon?: ReactNode
  startIcon?: ReactNode
  disabled?: boolean
  autofocus?: boolean
  helperText?: string
  inputmode?: "search" | "text" | "email" | "tel" | "url" | "none" | "numeric" | "decimal"
  caretPosition?: [number, number]
  onChange?: (value: string) => void
  onEnter?: () => void
  onKeyDown?: (e: KeyboardEvent) => void
  onFocus?: () => void
  onBlur?: () => void
  onChangeCaret?: (position: [number, number]) => void
}

const TextField: FC<TextFieldProps> = ({
  label,
  value,
  type = "text",
  error = "",
  warning = "",
  helperText = "",
  placeholder,
  endIcon,
  startIcon,
  disabled = false,
  autofocus = false,
  inputmode = "text",
  caretPosition,
  onChange,
  onEnter,
  onKeyDown,
  onFocus,
  onBlur,
  onChangeCaret,
}) => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();

  const inputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fieldRef = useMemo(() => type === "textarea" ? textareaRef : inputRef, [type]);
  const [selectionPos, setSelectionPos] = useState<[start: number, end: number]>([0, 0]);

  const inputClasses = classNames({
    "vm-text-field__input": true,
    "vm-text-field__input_error": error,
    "vm-text-field__input_warning": !error && warning,
    "vm-text-field__input_icon-start": startIcon,
    "vm-text-field__input_disabled": disabled,
    "vm-text-field__input_textarea": type === "textarea",
  });

  const updateCaretPosition = (target: HTMLInputElement | HTMLTextAreaElement) => {
    const { selectionStart, selectionEnd } = target;
    setSelectionPos([selectionStart || 0, selectionEnd || 0]);
  };

  const handleMouseUp = (e: MouseEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    updateCaretPosition(e.currentTarget);
  };

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

  const handleKeyUp = (e: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    updateCaretPosition(e.currentTarget);
  };

  const handleChange = (e: FormEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    if (disabled) return;
    onChange && onChange(e.currentTarget.value);
    updateCaretPosition(e.currentTarget);
  };

  const handleFocus = () => {
    onFocus && onFocus();
  };

  const handleBlur = () => {
    onBlur && onBlur();
  };

  const setSelectionRange = (range: [number, number]) => {
    try {
      fieldRef.current && fieldRef.current.setSelectionRange(range[0], range[1]);
    }  catch (e) {
      return e;
    }
  };

  useEffect(() => {
    if (!autofocus || isMobile) return;
    fieldRef?.current?.focus && fieldRef.current.focus();
  }, [fieldRef, autofocus]);

  useEffect(() => {
    onChangeCaret && onChangeCaret(selectionPos);
  }, [selectionPos]);

  useEffect(() => {
    setSelectionRange(selectionPos);
  }, [value]);

  useEffect(() => {
    caretPosition && setSelectionRange(caretPosition);
  }, [caretPosition]);

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
          onKeyUp={handleKeyUp}
          onFocus={handleFocus}
          onBlur={handleBlur}
          onMouseUp={handleMouseUp}
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
          onKeyUp={handleKeyUp}
          onFocus={handleFocus}
          onBlur={handleBlur}
          onMouseUp={handleMouseUp}
        />
      )
    }
    {label && <span className="vm-text-field__label">{label}</span>}
    <TextFieldMessage
      error={error}
      warning={warning}
      info={helperText}
    />
  </label>
  ;
};

export default TextField;
