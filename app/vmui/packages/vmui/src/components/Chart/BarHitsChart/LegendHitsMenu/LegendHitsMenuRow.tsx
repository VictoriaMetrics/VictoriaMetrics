import React, { FC, useRef, useState } from "preact/compat";
import classNames from "classnames";
import { ReactNode, useEffect } from "react";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { LegendLogHitsMenu } from "../../../../api/types";
import { ArrowDropDownIcon } from "../../../Main/Icons";
import useClickOutside from "../../../../hooks/useClickOutside";

interface Props {
  title: string | ReactNode;
  handler?: () => void;
  iconStart?: ReactNode;
  iconEnd?: ReactNode;
  className?: string;
  submenu?: LegendLogHitsMenu[];
}

const LegendHitsMenuRow: FC<Props> = ({ title, handler, iconStart, iconEnd, className, submenu }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const titleRef = useRef<HTMLDivElement>(null);
  const submenuRef = useRef<HTMLDivElement>(null);

  const [isOverflownTitle, setIsOverflownTitle] = useState(false);

  const [openSubmenu, setOpenSubmenu] = useState(false);
  const [posSubmenuLeft, setPosSubmenuLeft] = useState(false);
  const hasSubmenu = !!submenu?.length;

  const handleToggleContextMenu = () => {
    setOpenSubmenu(prev => !prev);
  };

  const handleCloseContextMenu = () => {
    setOpenSubmenu(false);
  };

  const handleClick = () => {
    handler && handler();
    hasSubmenu && handleToggleContextMenu();
  };


  useEffect(() => {
    if (!titleRef.current) return;
    setIsOverflownTitle(titleRef.current.scrollWidth > titleRef.current.clientWidth);
  }, [title, titleRef]);

  useEffect(() => {
    requestAnimationFrame(() => {
      if (!openSubmenu || !submenuRef.current) {
        setPosSubmenuLeft(false);
        return;
      }

      const { left, width } = submenuRef.current.getBoundingClientRect();
      setPosSubmenuLeft(left + width > window.innerWidth);
    });
  }, [submenuRef, openSubmenu]);

  useClickOutside(containerRef, handleCloseContextMenu);

  const titleContent = (
    <div
      ref={titleRef}
      className="vm-legend-hits-menu-row__title"
    >
      {title}
    </div>
  );

  return (
    <div
      ref={containerRef}
      className={classNames({
        "vm-legend-hits-menu-row": true,
        "vm-legend-hits-menu-row_interactive": !!handler || hasSubmenu,
        [`${className}`]: className
      })}
      onClick={handleClick}
    >
      {iconStart && <div className="vm-legend-hits-menu-row__icon">{iconStart}</div>}
      {isOverflownTitle ? (<Tooltip title={title}>{titleContent}</Tooltip>) : titleContent}
      {iconEnd && !hasSubmenu && <div className="vm-legend-hits-menu-row__icon">{iconEnd}</div>}

      {hasSubmenu && (
        <div className="vm-legend-hits-menu-row__icon vm-legend-hits-menu-row__icon_drop">
          <ArrowDropDownIcon/>
        </div>
      )}

      {openSubmenu && submenu && (
        <div
          ref={submenuRef}
          className={classNames({
            "vm-legend-hits-menu": true,
            "vm-legend-hits-menu_submenu": true,
            "vm-legend-hits-menu_submenu_left": posSubmenuLeft
          })}
        >
          <div className="vm-legend-hits-menu-section">
            {submenu.map(({ icon, title, handler }) => (
              <LegendHitsMenuRow
                key={title}
                iconStart={icon}
                title={title}
                handler={handler}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default LegendHitsMenuRow;
