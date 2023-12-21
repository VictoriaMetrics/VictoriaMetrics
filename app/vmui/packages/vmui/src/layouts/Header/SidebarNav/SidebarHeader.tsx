import React, { FC, useEffect, useRef } from "preact/compat";
import { useLocation } from "react-router-dom";
import ShortcutKeys from "../../../components/Main/ShortcutKeys/ShortcutKeys";
import classNames from "classnames";
import HeaderNav from "../HeaderNav/HeaderNav";
import useClickOutside from "../../../hooks/useClickOutside";
import MenuBurger from "../../../components/Main/MenuBurger/MenuBurger";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import "./style.scss";
import useBoolean from "../../../hooks/useBoolean";
import { AppType } from "../../../types/appType";

interface SidebarHeaderProps {
  background: string
  color: string
}

const { REACT_APP_TYPE } = process.env;
const isLogsApp = REACT_APP_TYPE === AppType.logs;

const SidebarHeader: FC<SidebarHeaderProps> = ({
  background,
  color,
}) => {
  const { pathname } = useLocation();
  const { isMobile } = useDeviceDetect();

  const sidebarRef = useRef<HTMLDivElement>(null);

  const {
    value: openMenu,
    toggle: handleToggleMenu,
    setFalse: handleCloseMenu,
  } = useBoolean(false);

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
        {!isMobile && !isLogsApp && <ShortcutKeys showTitle={true}/>}
      </div>
    </div>
  </div>;
};

export default SidebarHeader;
