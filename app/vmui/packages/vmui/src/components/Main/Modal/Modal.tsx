import React, { FC, useEffect } from "preact/compat";
import ReactDOM from "react-dom";
import { CloseIcon } from "../Icons";
import Button from "../Button/Button";
import { ReactNode } from "react";
import "./style.scss";

interface ModalProps {
  title?: string
  children: ReactNode
  onClose: () => void
}

const Modal: FC<ModalProps> = ({ title, children, onClose }) => {

  const handleKeyUp = (e: globalThis.KeyboardEvent) => {
    if (e.key === "Escape") onClose();
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
      <div className="vm-modal-header">
        {title && (
          <div className="vm-modal-header__title">
            {title}
          </div>
        )}
        <div className="vm-modal-header__close">
          <Button
            size="small"
            onClick={onClose}
          >
            <CloseIcon/>
          </Button>
        </div>
      </div>
      <div
        className="vm-modal-content"
        onMouseDown={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </div>
  ), document.body);
};

export default Modal;
