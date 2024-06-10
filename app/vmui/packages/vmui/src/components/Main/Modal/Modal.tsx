import React, { FC, useCallback, useEffect } from "preact/compat";
import ReactDOM from "react-dom";
import { CloseIcon } from "../Icons";
import Button from "../Button/Button";
import { ReactNode, MouseEvent } from "react";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { useLocation, useNavigate } from "react-router-dom";
import useEventListener from "../../../hooks/useEventListener";

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
  isOpen = true
}) => {
  const { isMobile } = useDeviceDetect();
  const navigate = useNavigate();
  const location = useLocation();

  const handleKeyUp = useCallback((e: KeyboardEvent) => {
    if (!isOpen) return;
    if (e.key === "Escape") onClose();
  }, [isOpen]);

  const handleMouseDown = (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
  };

  const handlePopstate = useCallback(() => {
    if (isOpen) {
      navigate(location, { replace: true });
      onClose();
    }
  }, [isOpen, location, onClose]);

  const handleDisplayModal = () => {
    if (!isOpen) return;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = "auto";
    };
  };

  useEffect(handleDisplayModal, [isOpen]);

  useEventListener("popstate", handlePopstate);
  useEventListener("keyup", handleKeyUp);

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
              ariaLabel="close"
            >
              <CloseIcon/>
            </Button>
          </div>
        </div>
        {/* tabIndex to fix Ctrl-A */}
        <div
          className="vm-modal-content-body"
          onMouseDown={handleMouseDown}
          tabIndex={0}
        >
          {children}
        </div>
      </div>
    </div>
  ), document.body);
};

export default Modal;
