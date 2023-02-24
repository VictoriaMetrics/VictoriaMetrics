import React, { FC, useEffect, useRef, useState } from "preact/compat";
import { useLocation } from "react-router-dom";
import ShortcutKeys from "../../../Main/ShortcutKeys/ShortcutKeys";
import classNames from "classnames";
import HeaderNav from "../HeaderNav/HeaderNav";
import useClickOutside from "../../../../hooks/useClickOutside";
import MenuBurger from "../../../Main/MenuBurger/MenuBurger";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import "./style.scss";

interface SidebarHeaderProps {
  background: string
  color: string
}

const SidebarHeader: FC<SidebarHeaderProps> = ({
  background,
  color,
}) => {
  const { pathname } = useLocation();
  const { isMobile } = useDeviceDetect();

  const sidebarRef = useRef<HTMLDivElement>(null);
  const [openMenu, setOpenMenu] = useState(false);

  const handleToggleMenu = () => {
    setOpenMenu(prev => !prev);
  };

  const handleCloseMenu = () => {
    setOpenMenu(false);
  };

  useEffect(handleCloseMenu, [pathname]);

  useClickOutside(sidebarRef, handleCloseMenu);

  return <div
    className="vm-header-sidebar"
    ref={sidebarRef}
  >
    <div
      className={classNames({
        "vm-header-sidebar-button": true,
        "vm-header-sidebar-button_open": openMenu
      })}
      onClick={handleToggleMenu}
    >
      <MenuBurger open={openMenu}/>
    </div>
    <div
      className={classNames({
        "vm-header-sidebar-menu": true,
        "vm-header-sidebar-menu_open": openMenu
      })}
    >
      <div>
        <HeaderNav
          color={color}
          background={background}
          direction="column"
        />
      </div>
      <div className="vm-header-sidebar-menu-settings">
        {!isMobile && <ShortcutKeys showTitle={true}/>}
      </div>
    </div>
  </div>;
};

export default SidebarHeader;
