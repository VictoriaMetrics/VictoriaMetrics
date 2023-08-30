import React, { FC, useEffect, useRef, useState } from "react";
import useEventListener from "../../../hooks/useEventListener";
import classNames from "classnames";
import "./style.scss";
import { useMemo } from "preact/compat";

interface TextFieldErrorProps {
  error: string;
  warning: string;
  info: string;
}

const TextFieldMessage: FC<TextFieldErrorProps> = ({ error, warning, info }) => {
  const messageRef = useRef<HTMLSpanElement>(null);
  const [isMessageTruncated, setIsMessageTruncated] = useState(false);
  const [showFull, setShowFull] = useState(false);

  const prefix = useMemo(() => {
    if (error) return "ERROR: ";
    if (warning) return "WARNING: ";
    return "";
  }, [error, warning]);

  const message = `${prefix}${error || warning || info}`;

  const checkIfTextTruncated = () => {
    const el = messageRef.current;
    if (el) {
      const { offsetWidth, scrollWidth, offsetHeight, scrollHeight } = el;
      // The "+1" is for the scrollbar in Firefox
      const overflowed = (offsetWidth + 1) < scrollWidth || (offsetHeight + 1) < scrollHeight;
      setIsMessageTruncated(overflowed);
    } else {
      setIsMessageTruncated(false);
    }
  };

  const handleClickError = () => {
    if (!isMessageTruncated) return;
    setShowFull(true);
    setIsMessageTruncated(false);
  };

  useEffect(() => {
    setShowFull(false);
    checkIfTextTruncated();
  }, [messageRef, message]);

  useEventListener("resize", checkIfTextTruncated);

  if (!error && !warning && !info) return null;

  return (
    <span
      className={classNames({
        "vm-text-field__error": true,
        "vm-text-field__warning": warning && !error,
        "vm-text-field__helper-text": !warning && !error,
        "vm-text-field__error_overflowed": isMessageTruncated,
        "vm-text-field__error_full": showFull,
      })}
      data-show={!!message}
      ref={messageRef}
      onClick={handleClickError}
    >
      {message}
    </span>
  );
};

export default TextFieldMessage;
