import React, { FC } from "preact/compat";
import { NavLink } from "react-router-dom";
import classNames from "classnames";

interface NavItemProps {
  activeMenu: string,
  label: string,
  value: string,
  color?: string
}

const NavItem: FC<NavItemProps> = ({
  activeMenu,
  label,
  value,
  color
}) => (
  <NavLink
    className={classNames({
      "vm-header-nav-item": true,
      "vm-header-nav-item_active": activeMenu === value // || m.submenu?.find(m => m.value === activeMenu)
    })}
    style={{ color }}
    to={value}
  >
    {label}
  </NavLink>
);

export default NavItem;
