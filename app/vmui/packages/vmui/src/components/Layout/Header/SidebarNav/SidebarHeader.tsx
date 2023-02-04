import React, { FC, useEffect, useRef, useState } from "preact/compat";
import GlobalSettings from "../../../Configurators/GlobalSettings/GlobalSettings";
import { useLocation } from "react-router-dom";
import ShortcutKeys from "../../../Main/ShortcutKeys/ShortcutKeys";
import { LogoFullIcon } from "../../../Main/Icons";
import classNames from "classnames";
import HeaderNav from "../HeaderNav/HeaderNav";
import useClickOutside from "../../../../hooks/useClickOutside";
import MenuBurger from "../../../Main/MenuBurger/MenuBurger";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import "./style.scss";

interface SidebarHeaderProps {
  background: string
  color: string
  onClickLogo: () => void
}

const SidebarHeader: FC<SidebarHeaderProps> = ({
  background,
  color,
  onClickLogo,
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
    >
      <MenuBurger
        open={openMenu}
        onClick={handleToggleMenu}
      />
    </div>
    <div
      className={classNames({
        "vm-header-sidebar-menu": true,
        "vm-header-sidebar-menu_open": openMenu
      })}
    >
      <div
        className="vm-header-sidebar-menu__logo"
        onClick={onClickLogo}
        style={{ color }}
      >
        <LogoFullIcon/>
      </div>
      <div>
        <HeaderNav
          color={color}
          background={background}
          direction="column"
        />
      </div>
      <div className="vm-header-sidebar-menu-settings">
        <GlobalSettings showTitle={true}/>
        {!isMobile && <ShortcutKeys showTitle={true}/>}
      </div>
    </div>
  </div>;
};

export default SidebarHeader;
