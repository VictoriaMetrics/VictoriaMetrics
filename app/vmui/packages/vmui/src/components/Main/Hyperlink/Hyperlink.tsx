import React, { FC } from "preact/compat";
import { ReactNode } from "react";
import classNames from "classnames";

interface Hyperlink {
  text?: string;
  href: string;
  children?: ReactNode;
  colored?: boolean;
  underlined?: boolean;
  withIcon?: boolean;
}

const Hyperlink: FC<Hyperlink> = ({
  text,
  href,
  children,
  colored = true,
  underlined = false,
  withIcon = false,
}) => (
  <a
    href={href}
    className={classNames({
      "vm-link": true,
      "vm-link_colored": colored,
      "vm-link_underlined": underlined,
      "vm-link_with-icon": withIcon,
    })}
    target="_blank"
    rel="noreferrer"
  >
    {text || children}
  </a>
);

export default Hyperlink;
