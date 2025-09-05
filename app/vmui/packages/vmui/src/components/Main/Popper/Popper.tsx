import {
  FC,
  MouseEvent as ReactMouseEvent,
  ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
  useCallback,
  createPortal
} from "react";
import classNames from "classnames";
import "./style.scss";
import useClickOutside from "../../../hooks/useClickOutside";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../Button/Button";
import { CloseIcon } from "../Icons";
import { useLocation, useNavigate } from "react-router-dom";
import useEventListener from "../../../hooks/useEventListener";

interface PopperProps {
  children: ReactNode
  open: boolean
  onClose: () => void
  buttonRef?: React.RefObject<HTMLElement>
  placement?: "bottom-right" | "bottom-left" | "top-left" | "top-right" | "center-left" | "center-right" | "fixed"
  placementPosition?: { top: number, left: number } | null
  animation?: string
  offset?: { top: number, left: number }
  clickOutside?: boolean,
  fullWidth?: boolean
  title?: string
  disabledFullScreen?: boolean
  variant?: "default" | "dark"
  classes?: string[]
}

const Popper: FC<PopperProps> = ({
  children,
  buttonRef,
  placement = "bottom-left",
  placementPosition,
  open = false,
  onClose,
  offset = { top: 6, left: 0 },
  clickOutside = true,
  fullWidth,
  title,
  disabledFullScreen,
  classes,
  variant
}) => {
  const { isMobile } = useDeviceDetect();
  const navigate = useNavigate();
  const location = useLocation();
  const [popperSize, setPopperSize] = useState({ width: 0, height: 0 });
  const [isOpen, setIsOpen] = useState(false);

  const popperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setIsOpen(open);

    if (!open && onClose) onClose();
    if (open && isMobile && !disabledFullScreen) {
      document.body.style.overflow = "hidden";
    }

    return () => {
      document.body.style.overflow = "auto";
    };
  }, [open]);

  useEffect(() => {
    setPopperSize({
      width: popperRef?.current?.clientWidth || 0,
      height: popperRef?.current?.clientHeight || 0
    });
    setIsOpen(false);
  }, [popperRef]);

  const popperStyle = useMemo(() => {
    if (!buttonRef) return {};
    const buttonEl = buttonRef.current;

    if (!buttonEl || !isOpen) return {};

    const buttonPos = buttonEl.getBoundingClientRect();

    const position = {
      top: 0,
      left: 0,
      width: "auto"
    };

    const needAlignRight = placement?.includes("right");
    const needAlignTop = placement?.includes("top");
    const needAlignCenter = placement?.includes("center");

    const offsetTop = offset?.top || 0;
    const offsetLeft = offset?.left || 0;

    position.left = position.left = buttonPos.left + offsetLeft;
    position.top = buttonPos.height + buttonPos.top + offsetTop;

    if (needAlignRight) position.left = buttonPos.right - popperSize.width;
    if (needAlignTop) position.top = buttonPos.top - popperSize.height - offsetTop;
    if (needAlignCenter) position.top = buttonPos.top + (buttonPos.height - popperSize.height) / 2 - offsetTop;

    if (placement === "fixed" && placementPosition) {
      position.top = Math.max(placementPosition.top + offset.top, 0);
      position.left = Math.max(placementPosition.left + offset.left, 0);
      return position;
    }

    const { innerWidth, innerHeight } = window;

    const isOverflowBottom = (position.top + popperSize.height) > innerHeight;
    const isOverflowTop = (position.top) < 0;
    const isOverflowRight = (position.left + popperSize.width) > innerWidth;
    const isOverflowLeft = (position.left) < 0;

    if (isOverflowBottom) position.top = buttonPos.top - popperSize.height - offsetTop;
    if (isOverflowTop) position.top = buttonPos.height + buttonPos.top + offsetTop;
    if (isOverflowRight) position.left = buttonPos.right - popperSize.width - offsetLeft;
    if (isOverflowLeft) position.left = buttonPos.left + offsetLeft;

    if (fullWidth) position.width = `${buttonPos.width}px`;
    if (position.top < 0) position.top = 0;
    if (position.left < 0) position.left = 0;

    return position;
  }, [buttonRef, placement, isOpen, children, fullWidth]);

  const handleClickClose = (e: ReactMouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    onClose();
  };

  const handleClose = () => {
    setIsOpen(false);
    onClose();
  };

  const handleClickOutside = () => {
    if (!clickOutside) return;
    handleClose();
  };

  useEffect(() => {
    if (!popperRef.current || !isOpen || (isMobile && !disabledFullScreen)) return;
    const { right, width } = popperRef.current.getBoundingClientRect();
    if (right > window.innerWidth) {
      const left = window.innerWidth - width;
      popperRef.current.style.left = `${left}px`;
    }
  }, [isOpen, popperRef, placementPosition]);

  const handlePopstate = useCallback(() => {
    if (isOpen && isMobile && !disabledFullScreen) {
      navigate(location, { replace: true });
      onClose();
    }
  }, [isOpen, isMobile, disabledFullScreen, location, onClose]);

  useEventListener("scroll", handleClose);
  useEventListener("popstate", handlePopstate);
  useClickOutside(popperRef, handleClickOutside, buttonRef);
  const classMap: Record<string, boolean> = {};
  (classes || []).forEach((cls) => {
    classMap[cls] = true;
  });

  return (
    <>
      {(isOpen || !popperSize.width) && createPortal((
        <div
          className={classNames({
            "vm-popper": true,
            [`vm-popper_${variant}`]: variant,
            "vm-popper_mobile": isMobile && !disabledFullScreen,
            "vm-popper_open": (isMobile || Object.keys(popperStyle).length) && isOpen,
            ...classMap,
          })}
          ref={popperRef}
          style={(isMobile && !disabledFullScreen) ? {} : popperStyle}
        >
          {(title || (isMobile && !disabledFullScreen)) && (
            <div className="vm-popper-header">
              <p className="vm-popper-header__title">{title}</p>
              <Button
                variant="text"
                color={variant === "dark" ? "white" : "primary"}
                size="small"
                onClick={handleClickClose}
                ariaLabel="close"
              >
                <CloseIcon/>
              </Button>
            </div>
          )}
          {children}
        </div>), document.body)}
    </>
  );
};

export default Popper;
