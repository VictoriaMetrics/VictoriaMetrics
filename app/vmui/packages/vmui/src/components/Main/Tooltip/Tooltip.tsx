import { Children, cloneElement, FC, isValidElement, useEffect, useMemo, useRef, useState, ReactNode } from "react";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { createPortal } from "react-dom";

interface TooltipProps {
  children: ReactNode
  title: ReactNode
  offset?: {top?: number, left?: number}
  open?: boolean
  placement?: "bottom-right" | "bottom-left" | "top-left" | "top-right" | "top-center" | "bottom-center"
}

const Tooltip: FC<TooltipProps> = ({
  children,
  title,
  open,
  placement = "bottom-center",
  offset = { top: 6, left: 0 }
}) => {
  const { isMobile } = useDeviceDetect();

  const [isOpen, setIsOpen] = useState(false);
  const [popperSize, setPopperSize] = useState({ width: 0, height: 0 });

  const buttonRef = useRef<HTMLDivElement>(null);
  const popperRef = useRef<HTMLDivElement>(null);

  const onScrollWindow = () => setIsOpen(false);

  useEffect(() => {
    if (!popperRef.current || !isOpen) return;
    setPopperSize({
      width: popperRef.current.clientWidth,
      height: popperRef.current.clientHeight
    });
    window.addEventListener("scroll", onScrollWindow);

    return () => {
      window.removeEventListener("scroll", onScrollWindow);
    };
  }, [isOpen, title]);

  const popperStyle = useMemo(() => {
    const buttonEl = buttonRef?.current;

    if (!buttonEl|| !isOpen) return {};
    const buttonPos = buttonEl.getBoundingClientRect();
    const position = { top: 0, left: 0 };

    const needAlignRight = placement === "bottom-right" || placement === "top-right";
    const needAlignLeft = placement === "bottom-left" || placement === "top-left";
    const needAlignTop = placement?.includes("top");

    const offsetTop = offset?.top || 0;
    const offsetLeft = offset?.left || 0;

    position.left = buttonPos.left - ((popperSize.width - buttonPos.width)/2) + offsetLeft;
    position.top = buttonPos.height + buttonPos.top + offsetTop;

    if (needAlignRight) position.left = buttonPos.right - popperSize.width;
    if (needAlignLeft) position.left = buttonPos.left + offsetLeft;
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

    if (position.top < 0) position.top = 20;
    if (position.left < 0) position.left = 20;

    return position;
  },[buttonRef, placement, isOpen, popperSize]);

  const handleMouseEnter = () => {
    if (typeof open === "boolean") return;
    setIsOpen(true);
  };

  const handleMouseLeave = () => {
    setIsOpen(false);
  };

  useEffect(() => {
    if (typeof open === "boolean") setIsOpen(open);
  }, [open]);

  useEffect(() => {
    const nodeEl = buttonRef?.current;
    if (!nodeEl) return;
    nodeEl.addEventListener("mouseenter", handleMouseEnter);
    nodeEl.addEventListener("mouseleave", handleMouseLeave);

    return () => {
      nodeEl.removeEventListener("mouseenter", handleMouseEnter);
      nodeEl.removeEventListener("mouseleave", handleMouseLeave);
    };
  }, [buttonRef]);

  const rawChild = Children.only(children);
  const childWithRef = isValidElement(rawChild)
    ? cloneElement(rawChild as never, { ref: buttonRef })
    : rawChild;

  return (
    <>
      {childWithRef}

      {!isMobile && isOpen && createPortal((
        <div
          className="vm-tooltip"
          ref={popperRef}
          style={popperStyle}
        >
          {title}
        </div>), document.body)}
    </>
  );
};

export default Tooltip;
