import React, { FC, useCallback, useState } from "preact/compat";
import Tooltip from "../Main/Tooltip/Tooltip";
import Button from "../Main/Button/Button";
import { DownloadIcon } from "../Main/Icons";
import Popper from "../Main/Popper/Popper";
import { useRef } from "react";
import "./style.scss";

interface DownloadButtonProps {
    title: string;
    downloadFormatOptions?: string[];
    onDownload: (format?: string) => void;
}

const DownloadButton: FC<DownloadButtonProps> = ({ title, downloadFormatOptions, onDownload }) => {
  const [isPopupOpen, setIsPopupOpen] = useState<boolean>(false);
  const onClosePopup = useCallback(() => setIsPopupOpen(false), []);
  const downloadButtonRef = useRef<HTMLDivElement>(null);
  const onDownloadClick = useCallback(() => {
    if(isPopupOpen){
      onClosePopup();
      return;
    }

    if(downloadFormatOptions && downloadFormatOptions.length > 0){
      setIsPopupOpen(true);
    } else {
      onDownload();
      onClosePopup();
    }
  }, [onDownload, onClosePopup, isPopupOpen,setIsPopupOpen]);

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
          {downloadFormatOptions.map((option) => <Button
            size={"small"}
            variant={"text"}
            onClick={onDownloadFormatClick}
            key={option}
            className={"vm-download-button__format-option"}
          >{option}</Button>)}
        </Popper>)}
    </>
  );
};

export default DownloadButton;