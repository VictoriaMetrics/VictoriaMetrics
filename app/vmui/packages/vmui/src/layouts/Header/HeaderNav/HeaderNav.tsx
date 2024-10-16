import React, { FC, useState } from "preact/compat";
import { useLocation } from "react-router-dom";
import { useEffect } from "react";
import "./style.scss";
import NavItem from "./NavItem";
import NavSubItem from "./NavSubItem";
import classNames from "classnames";
import useNavigationMenu from "../../../router/useNavigationMenu";
import { NavigationItemType } from "../../../router/navigation";

interface HeaderNavProps {
  color: string
  background: string
  direction?: "row" | "column"
}

const HeaderNav: FC<HeaderNavProps> = ({ color, background, direction }) => {
  const { pathname } = useLocation();
  const [activeMenu, setActiveMenu] = useState(pathname);
  const menu = useNavigationMenu();

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
              type={m.type || NavigationItemType.internalLink}
            />
          )
      ))}
    </nav>
  );
};

export default HeaderNav;
