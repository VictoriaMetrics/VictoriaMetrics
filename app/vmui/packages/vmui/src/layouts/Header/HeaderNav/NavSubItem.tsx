import React, { FC, useRef, useState } from "preact/compat";
import { useLocation } from "react-router-dom";
import classNames from "classnames";
import { ArrowDropDownIcon } from "../../../components/Main/Icons";
import Popper from "../../../components/Main/Popper/Popper";
import NavItem from "./NavItem";
import { useEffect } from "react";
import useBoolean from "../../../hooks/useBoolean";
import { NavigationItem, NavigationItemType } from "../../../constants/navigation";

interface NavItemProps {
  activeMenu: string,
  label: string,
  submenu: NavigationItem[],
  color?: string
  background?: string
  direction?: "row" | "column"
}

const NavSubItem: FC<NavItemProps> = ({
  activeMenu,
  label,
  color,
  background,
  submenu,
  direction
}) => {
  const { pathname } = useLocation();

  const [menuTimeout, setMenuTimeout] = useState<NodeJS.Timeout | null>(null);
  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openSubmenu,
    setFalse: handleCloseSubmenu,
    setTrue: setOpenSubmenu,
  } = useBoolean(false);

  const handleOpenSubmenu = () => {
    setOpenSubmenu();
    if (menuTimeout) clearTimeout(menuTimeout);
  };

  const handleMouseLeave = () => {
    if (menuTimeout) clearTimeout(menuTimeout);
    const timeout = setTimeout(handleCloseSubmenu, 300);
    setMenuTimeout(timeout);
  };

  const handleMouseEnterPopup = () => {
    if (menuTimeout) clearTimeout(menuTimeout);
  };

  useEffect(() => {
    handleCloseSubmenu();
  }, [pathname]);

  if (direction === "column") {
    return (
      <>
        {submenu.map(sm => (
          <NavItem
            key={sm.value}
            activeMenu={activeMenu}
            value={sm.value || ""}
            label={sm.label || ""}
            type={sm.type || NavigationItemType.internalLink}
          />
        ))}
      </>
    );
  }

  return (
    <div
      className={classNames({
        "vm-header-nav-item": true,
        "vm-header-nav-item_sub": true,
        "vm-header-nav-item_open": openSubmenu,
        "vm-header-nav-item_active": submenu.find(m => m.value === activeMenu)
      })}
      style={{ color }}
      onMouseEnter={handleOpenSubmenu}
      onMouseLeave={handleMouseLeave}
      ref={buttonRef}
    >
      {label}
      <ArrowDropDownIcon/>

      <Popper
        open={openSubmenu}
        placement="bottom-left"
        offset={{ top: 12, left: 0 }}
        onClose={handleCloseSubmenu}
        buttonRef={buttonRef}
      >
        <div
          className="vm-header-nav-item-submenu"
          style={{ background }}
          onMouseLeave={handleMouseLeave}
          onMouseEnter={handleMouseEnterPopup}
        >
          {submenu.map(sm => (
            <NavItem
              key={sm.value}
              activeMenu={activeMenu}
              value={sm.value || ""}
              label={sm.label || ""}
              color={color}
              type={sm.type || NavigationItemType.internalLink}
            />
          ))}
        </div>
      </Popper>
    </div>
  );
};

export default NavSubItem;
