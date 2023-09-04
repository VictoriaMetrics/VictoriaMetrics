import React, { FC, useMemo, useState } from "preact/compat";
import router, { routerOptions } from "../../../router";
import { getAppModeEnable } from "../../../utils/app-mode";
import { useLocation } from "react-router-dom";
import { useDashboardsState } from "../../../state/dashboards/DashboardsStateContext";
import { useEffect } from "react";
import "./style.scss";
import NavItem from "./NavItem";
import NavSubItem from "./NavSubItem";
import classNames from "classnames";
import { defaultNavigation, logsNavigation } from "../../../constants/navigation";

interface HeaderNavProps {
  color: string
  background: string
  direction?: "row" | "column"
}

const HeaderNav: FC<HeaderNavProps> = ({ color, background, direction }) => {
  const { REACT_APP_LOGS } = process.env;
  const appModeEnable = getAppModeEnable();
  const { dashboardsSettings } = useDashboardsState();
  const { pathname } = useLocation();

  const [activeMenu, setActiveMenu] = useState(pathname);

  const menu = useMemo(() => REACT_APP_LOGS ? logsNavigation : ([
    ...defaultNavigation,
    {
      label: routerOptions[router.dashboards].title,
      value: router.dashboards,
      hide: appModeEnable || !dashboardsSettings.length,
    }
  ].filter(r => !r.hide)), [appModeEnable, dashboardsSettings]);

  useEffect(() => {
    setActiveMenu(pathname);
  }, [pathname]);


  return (
    <nav
      className={classNames({
        "vm-header-nav": true,
        [`vm-header-nav_${direction}`]: direction
      })}
    >
      {menu.map(m => (
        m.submenu
          ? (
            <NavSubItem
              key={m.label}
              activeMenu={activeMenu}
              label={m.label || ""}
              submenu={m.submenu}
              color={color}
              background={background}
              direction={direction}
            />
          )
          : (
            <NavItem
              key={m.value}
              activeMenu={activeMenu}
              value={m.value || ""}
              label={m.label || ""}
              color={color}
            />
          )
      ))}
    </nav>
  );
};

export default HeaderNav;
