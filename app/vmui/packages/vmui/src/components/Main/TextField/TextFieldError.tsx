import React, { FC, useEffect, useRef, useState } from "react";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import useEventListener from "../../../hooks/useEventListener";
import Tooltip from "../Tooltip/Tooltip";
import "./style.scss";

interface TextFieldErrorProps {
  error: string;
}

const TextFieldError: FC<TextFieldErrorProps> = ({ error }) => {
  const copyToClipboard = useCopyToClipboard();
  const errorRef = useRef<HTMLSpanElement>(null);
  const [isErrorTruncated, setIsErrorTruncated] = useState(false);
  const [isCopied, setIsCopied] = useState(false);

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

  const handleCopyError = async () => {
    const copied = await copyToClipboard(error);
    setIsCopied(copied);
  };

  useEffect(() => {
    if (!isCopied) return;
    const timeout = setTimeout(() => setIsCopied(false), 3000);
    return () => clearTimeout(timeout);
  }, [isCopied]);

  useEffect(checkIfTextTruncated, [errorRef, error]);
  useEventListener("resize", checkIfTextTruncated);

  const contentError = (
    <span
      className="vm-text-field__error"
      data-show={!!error}
      ref={errorRef}
      onClick={handleCopyError}
    >
      {error}
    </span>
  );

  if (isErrorTruncated) {
    return (
      <Tooltip
        title={(
          <p className="vm-text-field__error-tooltip">
            {isCopied ? "error text has been copied" : error}
          </p>
        )}
      >
        {contentError}
      </Tooltip>
    );
  }

  return contentError;
};

export default TextFieldError;
