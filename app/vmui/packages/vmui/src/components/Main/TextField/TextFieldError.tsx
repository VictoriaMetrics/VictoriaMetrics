import React, { FC, useEffect, useRef, useState } from "react";
import useEventListener from "../../../hooks/useEventListener";
import classNames from "classnames";
import "./style.scss";

interface TextFieldErrorProps {
  error: string;
}

const TextFieldError: FC<TextFieldErrorProps> = ({ error }) => {
  const errorRef = useRef<HTMLSpanElement>(null);
  const [isErrorTruncated, setIsErrorTruncated] = useState(false);
  const [showFull, setShowFull] = useState(false);

  const checkIfTextTruncated = () => {
    const el = errorRef.current;
    if (el) {
      const { offsetWidth, scrollWidth, offsetHeight, scrollHeight } = el;
      // The "+1" is for the scrollbar in Firefox
      const overflowed = (offsetWidth + 1) < scrollWidth || (offsetHeight + 1) < scrollHeight;
      setIsErrorTruncated(overflowed);
    } else {
      setIsErrorTruncated(false);
    }
  };

  const handleClickError = () => {
    if (!isErrorTruncated) return;
    setShowFull(true);
    setIsErrorTruncated(false);
  };

  useEffect(() => {
    setShowFull(false);
    checkIfTextTruncated();
  }, [errorRef, error]);

  useEventListener("resize", checkIfTextTruncated);

  return (
    <span
      className={classNames({
        "vm-text-field__error": true,
        "vm-text-field__error_overflowed": isErrorTruncated,
        "vm-text-field__error_full": showFull,
      })}
      data-show={!!error}
      ref={errorRef}
      onClick={handleClickError}
    >
      {error}
    </span>
  );
};

export default TextFieldError;
