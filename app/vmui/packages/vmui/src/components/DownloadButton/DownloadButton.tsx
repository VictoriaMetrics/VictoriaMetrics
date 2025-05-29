import React, { FC, useCallback } from "preact/compat";
import Tooltip from "../Main/Tooltip/Tooltip";
import Button from "../Main/Button/Button";
import { DownloadIcon } from "../Main/Icons";
import Popper from "../Main/Popper/Popper";
import { useRef } from "react";
import "./style.scss";
import useBoolean from "../../hooks/useBoolean";

interface DownloadButtonProps {
  title: string;
  downloadFormatOptions?: string[];
  onDownload: (format?: string) => void;
}

const DownloadButton: FC<DownloadButtonProps> = ({ title, downloadFormatOptions, onDownload }) => {
  const {
    value: isPopupOpen,
    setTrue: onOpenPopup,
    setFalse: onClosePopup,
  } = useBoolean(false);
  const downloadButtonRef = useRef<HTMLDivElement>(null);
  const onDownloadClick = useCallback(() => {
    if (isPopupOpen) {
      onClosePopup();
      return;
    }

    if (downloadFormatOptions && downloadFormatOptions.length > 0) {
      onOpenPopup();
    } else {
      onDownload();
      onClosePopup();
    }
  }, [onDownload, onClosePopup, isPopupOpen, onOpenPopup]);

  const onDownloadFormatClick = useCallback((event: Event) => {
    const button = event.currentTarget as HTMLButtonElement;
    onDownload(button.textContent ?? undefined);
  }, [onDownload]);

  return (
    <>
      <div ref={downloadButtonRef}>
        <Tooltip
          title={title}
        >
          <Button
            variant="text"
            startIcon={<DownloadIcon/>}
            onClick={onDownloadClick}
            ariaLabel={title}
          />
        </Tooltip>
      </div>
      {downloadFormatOptions && downloadFormatOptions.length > 0 && (
        <Popper
          open={isPopupOpen}
          onClose={onClosePopup}
          buttonRef={downloadButtonRef}
          placement={"bottom-right"}
        >
          {downloadFormatOptions.map((option) =>
            <div
              key={option}
              className={"vm-download-button__format-option"}
            >
              <Button
                variant="text"
                onClick={onDownloadFormatClick}
                className={"vm-download-button__format-option-button"}
              >
                {option}
              </Button>
            </div>)}
        </Popper>)}
    </>
  );
};

export default DownloadButton;