import React, { FC, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import classNames from "classnames";
import ReactDOM from "react-dom";
import "./style.scss";
import useClickOutside from "../../../hooks/useClickOutside";

interface PopperProps {
  children: ReactNode
  open: boolean
  onClose: () => void
  buttonRef: React.RefObject<HTMLDivElement>
  placement?: string
  animation?: string
  offset?: {top: number, left: number}
}

const Popper: FC<PopperProps> = ({
  children,
  buttonRef,
  placement = "bottom-left",
  open = false,
  onClose,
  animation,
  offset = { top: 0, left: 0 }
}) => {

  const [isOpen, setIsOpen] = useState(true);
  const [popperSize, setPopperSize] = useState({ width: 0, height: 0 });

  const popperRef = useRef<HTMLDivElement>(null);

  const onScrollWindow = () => {
    setIsOpen(false);
  };

  useEffect(() => {
    window.addEventListener("scroll", onScrollWindow);

    return () => {
      window.removeEventListener("scroll", onScrollWindow);
    };
  }, []);

  useEffect(() => {
    setIsOpen(open);
  }, [open]);

  useEffect(() => {
    if (!isOpen && onClose) onClose();
  }, [isOpen]);

  useEffect(() => {
    setPopperSize({
      width: popperRef?.current?.clientWidth || 0,
      height: popperRef?.current?.clientHeight || 0
    });
    setIsOpen(false);
  }, [popperRef]);

  const popperStyle = useMemo(() => {
    const buttonEl = buttonRef.current;

    if (!buttonEl|| !isOpen) return {};

    const buttonPos = buttonEl.getBoundingClientRect();

    const position = {
      top: 0,
      left: 0
    };

    if (placement === "bottom-right") {
      const top = buttonPos.height + buttonPos.top + 4 + offset.top;
      const left = buttonPos.right - popperSize.width + offset.left;
      position.top = top;
      position.left = left;
    }

    if (placement === "bottom-left") {
      const top = buttonPos.height + buttonPos.top + 4 + offset.top;
      const left = buttonPos.left + offset.left;
      position.top = top;
      position.left = left;
    }

    if (placement === "top-left") {
      const top = buttonPos.top - popperSize.height - 4 - offset.top;
      const left = buttonPos.left + offset.left;
      position.top = top;
      position.left = left;
    }

    if (placement === "top-right") {
      const top = buttonPos.top - popperSize.height - 4 - offset.top;
      const left = buttonPos.right - popperSize.width + offset.left;
      position.top = top;
      position.left = left;
    }

    const { innerWidth, innerHeight, scrollY, scrollX } = window;
    const margin = 20;
    const isOverflowTop = position.top + buttonPos.top + margin < (innerHeight + scrollY);
    const isOverflowRight = position.left + popperSize.width + margin > (innerWidth + scrollX);

    if (isOverflowTop) {
      position.top = buttonPos.height + buttonPos.top + 4 + offset.top;
    }

    if (isOverflowRight) {
      position.left = buttonPos.right - popperSize.width;
    }

    return position;
  },[buttonRef, placement, isOpen, children]);

  useClickOutside(popperRef, () => setIsOpen(false), buttonRef);

  const popperClasses = classNames({
    "vm-popper": true,
    "vm-popper_open": isOpen,
    [`vm-popper_open_${animation}`]: animation,
  });

  console.log(popperStyle);

  return (
    <>
      {isOpen && ReactDOM.createPortal((
        <div
          className={popperClasses}
          ref={popperRef}
          style={popperStyle}
        >
          {children}
        </div>), document.body)}
    </>
  );
};

export default Popper;
