import React, { FC, useMemo, useState } from "preact/compat";
import router, { routerOptions } from "../../../../router";
import { getAppModeEnable } from "../../../../utils/app-mode";
import { useLocation } from "react-router-dom";
import { useDashboardsState } from "../../../../state/dashboards/DashboardsStateContext";
import { useEffect } from "react";
import "./style.scss";
import NavItem from "./NavItem";
import NavSubItem from "./NavSubItem";

interface HeaderNavProps {
  color: string
  background: string
}

const HeaderNav: FC<HeaderNavProps> = ({ color, background }) => {
  const appModeEnable = getAppModeEnable();
  const { dashboardsSettings } = useDashboardsState();
  const { pathname } = useLocation();

  const [activeMenu, setActiveMenu] = useState(pathname);

  const menu = useMemo(() => ([
    {
      label: routerOptions[router.home].title,
      value: router.home,
    },
    {
      label: "Explore",
      submenu: [
        {
          label: routerOptions[router.metrics].title,
          value: router.metrics,
        },
        {
          label: routerOptions[router.cardinality].title,
          value: router.cardinality,
        },
        {
          label: routerOptions[router.topQueries].title,
          value: router.topQueries,
        },
      ]
    },
    {
      label: routerOptions[router.trace].title,
      value: router.trace,
    },
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
    <nav className="vm-header-nav">
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
            />
          )
          : (
            <NavItem
              key={m.value}
              activeMenu={activeMenu}
              value={m.value}
              label={m.label || ""}
              color={color}
            />
          )
      ))}
    </nav>
  );
};

export default HeaderNav;
