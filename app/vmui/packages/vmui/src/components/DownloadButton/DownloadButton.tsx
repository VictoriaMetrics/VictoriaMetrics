import { useCallback, useRef } from "preact/compat";
import Tooltip from "../Main/Tooltip/Tooltip";
import Button from "../Main/Button/Button";
import { DownloadIcon } from "../Main/Icons";
import Popper from "../Main/Popper/Popper";
import "./style.scss";
import useBoolean from "../../hooks/useBoolean";

interface DownloadButtonProps<T extends string> {
  title: string;
  downloadFormatOptions?: T[];
  onDownload: (format?: T) => void;
}

const DownloadButton = <T extends string>({ title, downloadFormatOptions, onDownload }: DownloadButtonProps<T>) => {
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

  const isDownloadFormat = useCallback((format: string): format is T => {
    return (downloadFormatOptions as string[])?.includes(format);
  }, [downloadFormatOptions]);

  const onDownloadFormatClick = useCallback((event: Event) => {
    const button = event.currentTarget as HTMLButtonElement;
    const format = button.textContent;
    if (format && isDownloadFormat(format)) {
      onDownload(format);
    } else {
      onDownload();
    }
    onClosePopup();
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
