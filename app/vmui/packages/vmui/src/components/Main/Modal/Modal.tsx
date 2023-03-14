import React, { FC, useEffect } from "preact/compat";
import ReactDOM from "react-dom";
import { CloseIcon } from "../Icons";
import Button from "../Button/Button";
import { ReactNode, MouseEvent } from "react";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { useLocation, useNavigate } from "react-router-dom";

interface ModalProps {
  title?: string
  children: ReactNode
  onClose: () => void
  className?: string
  isOpen?: boolean
}

const Modal: FC<ModalProps> = ({
  title,
  children,
  onClose,
  className,
  isOpen= true
}) => {
  const { isMobile } = useDeviceDetect();
  const navigate = useNavigate();
  const location = useLocation();

  const handleKeyUp = (e: KeyboardEvent) => {
    if (e.key === "Escape") onClose();
  };

  const handleMouseDown = (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
  };

  const handlePopstate = () => {
    if (isOpen) {
      navigate(location, { replace: true });
      onClose();
    }
  };

  useEffect(() => {
    window.addEventListener("popstate", handlePopstate);

    return () => {
      window.removeEventListener("popstate", handlePopstate);
    };
  }, [isOpen, location]);

  const handleDisplayModal = () => {
    if (!isOpen) return;
    document.body.style.overflow = "hidden";
    window.addEventListener("keyup", handleKeyUp);

    return () => {
      document.body.style.overflow = "auto";
      window.removeEventListener("keyup", handleKeyUp);
    };
  };

  useEffect(handleDisplayModal, [isOpen]);

  return ReactDOM.createPortal((
    <div
      className={classNames({
        "vm-modal": true,
        "vm-modal_mobile": isMobile,
        [`${className}`]: className
      })}
      onMouseDown={onClose}
    >
      <div className="vm-modal-content">
        <div
          className="vm-modal-content-header"
          onMouseDown={handleMouseDown}
        >
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
