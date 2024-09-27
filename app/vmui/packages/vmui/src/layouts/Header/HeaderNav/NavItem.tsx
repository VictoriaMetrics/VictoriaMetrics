import React, { FC } from "preact/compat";
import { NavLink } from "react-router-dom";
import classNames from "classnames";
import { NavigationItemType } from "../../../constants/navigation";

interface NavItemProps {
  activeMenu: string,
  label: string,
  value: string,
  type: NavigationItemType,
  color?: string,
}

const NavItem: FC<NavItemProps> = ({
  activeMenu,
  label,
  value,
  type,
  color
}) => {
  if (type === NavigationItemType.externalLink) return (
    <a
      className={classNames({
        "vm-header-nav-item": true,
        "vm-header-nav-item_active": activeMenu === value
      })}
      style={{ color }}
      href={value}
      target={"_blank"}
      rel="noreferrer"
    >
      {label}
    </a>
  );
  return (
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
};

export default NavItem;
