import React, { FC, useEffect } from "preact/compat";
import ReactDOM from "react-dom";
import { CloseIcon } from "../Icons";
import Button from "../Button/Button";
import { ReactNode, MouseEvent } from "react";
import "./style.scss";

interface ModalProps {
  title?: string
  children: ReactNode
  onClose: () => void
}

const Modal: FC<ModalProps> = ({ title, children, onClose }) => {

  const handleKeyUp = (e: KeyboardEvent) => {
    if (e.key === "Escape") onClose();
  };

  const handleMouseDown = (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
  };

  useEffect(() => {
    window.addEventListener("keyup", handleKeyUp);

    return () => {
      window.removeEventListener("keyup", handleKeyUp);
    };
  }, []);

  return ReactDOM.createPortal((
    <div
      className="vm-modal"
      onMouseDown={onClose}
    >
      <div className="vm-modal-content">
        <div className="vm-modal-content-header">
          {title && (
            <div className="vm-modal-content-header__title">
              {title}
            </div>
          )}
          <div className="vm-modal-header__close">
            <Button
              variant="text"
              size="small"
              onClick={onClose}
            >
              <CloseIcon/>
            </Button>
          </div>
        </div>
        <div
          className="vm-modal-content-body"
          onMouseDown={handleMouseDown}
        >
          {children}
        </div>
      </div>
    </div>
  ), document.body);
};

export default Modal;
