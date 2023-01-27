import React, { FC, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import classNames from "classnames";
import ReactDOM from "react-dom";
import "./style.scss";
import useClickOutside from "../../../hooks/useClickOutside";

interface PopperProps {
  children: ReactNode
  open: boolean
  onClose: () => void
  buttonRef: React.RefObject<HTMLElement>
  placement?: "bottom-right" | "bottom-left" | "top-left" | "top-right"
  animation?: string
  offset?: {top: number, left: number}
  clickOutside?: boolean,
  fullWidth?: boolean
}

const Popper: FC<PopperProps> = ({
  children,
  buttonRef,
  placement = "bottom-left",
  open = false,
  onClose,
  offset = { top: 6, left: 0 },
  clickOutside = true,
  fullWidth
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
      left: 0,
      width: "auto"
    };

    const needAlignRight = placement === "bottom-right" || placement === "top-right";
    const needAlignTop = placement?.includes("top");

    const offsetTop = offset?.top || 0;
    const offsetLeft = offset?.left || 0;

    position.left = position.left = buttonPos.left + offsetLeft;
    position.top = buttonPos.height + buttonPos.top + offsetTop;

    if (needAlignRight) position.left = buttonPos.right - popperSize.width;
    if (needAlignTop) position.top = buttonPos.top - popperSize.height - offsetTop;

    const { innerWidth, innerHeight } = window;
    const margin = 20;

    const isOverflowBottom = (position.top + popperSize.height + margin) > innerHeight;
    const isOverflowTop = (position.top - margin) < 0;
    const isOverflowRight = (position.left + popperSize.width + margin) > innerWidth;
    const isOverflowLeft = (position.left - margin) < 0;

    if (isOverflowBottom) position.top = buttonPos.top - popperSize.height - offsetTop;
    if (isOverflowTop) position.top = buttonPos.height + buttonPos.top + offsetTop;
    if (isOverflowRight) position.left = buttonPos.right - popperSize.width - offsetLeft;
    if (isOverflowLeft) position.left = buttonPos.left + offsetLeft;

    if (fullWidth) position.width = `${buttonPos.width}px`;

    return position;
  },[buttonRef, placement, isOpen, children, fullWidth]);

  if (clickOutside) useClickOutside(popperRef, () => setIsOpen(false), buttonRef);

  const popperClasses = classNames({
    "vm-popper": true,
    "vm-popper_open": isOpen,
  });

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
