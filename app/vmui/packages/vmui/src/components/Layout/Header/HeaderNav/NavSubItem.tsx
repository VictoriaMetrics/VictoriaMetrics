import React, { FC, useRef, useState } from "preact/compat";
import { useLocation } from "react-router-dom";
import classNames from "classnames";
import { ArrowDropDownIcon } from "../../../Main/Icons";
import Popper from "../../../Main/Popper/Popper";
import NavItem from "./NavItem";
import { useEffect } from "react";

interface NavItemProps {
  activeMenu: string,
  label: string,
  submenu: {label: string | undefined, value: string}[],
  color?: string
  background?: string
}

const NavSubItem: FC<NavItemProps> = ({
  activeMenu,
  label,
  color,
  background,
  submenu
}) => {
  const { pathname } = useLocation();

  const [openSubmenu, setOpenSubmenu] = useState(false);
  const [menuTimeout, setMenuTimeout] = useState<NodeJS.Timeout | null>(null);
  const buttonRef = useRef<HTMLDivElement>(null);

  const handleOpenSubmenu = () => {
    setOpenSubmenu(true);
    if (menuTimeout) clearTimeout(menuTimeout);
  };

  const handleCloseSubmenu = () => {
    setOpenSubmenu(false);
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
              value={sm.value}
              label={sm.label || ""}
            />
          ))}
        </div>
      </Popper>
    </div>
  );
};

export default NavSubItem;
