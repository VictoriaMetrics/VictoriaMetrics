import { ReactNode } from "react";
import React, { FC } from "preact/compat";
import { NavLink } from "react-router-dom";

interface TabItemWrapperProps {
  to: string
  isNavLink?: boolean
  className: string
  style: { color: string }
  children: ReactNode
  onClick: () => void
}

const TabItemWrapper: FC<TabItemWrapperProps> = ({ to, isNavLink, children, ...props }) => {
  if (isNavLink) {
    return (
      <NavLink
        to={to}
        {...props}
      >
        {children}
      </NavLink>
    );
  }

  return <div {...props}>{children}</div>;
};

export default TabItemWrapper;
