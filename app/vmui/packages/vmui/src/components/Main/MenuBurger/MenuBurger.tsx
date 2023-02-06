import React from "preact/compat";
import classNames from "classnames";
import "./style.scss";

const MenuBurger = ({ open, onClick }: {open: boolean, onClick: () => void}) => (
  <button
    className={classNames({
      "vm-menu-burger": true,
      "vm-menu-burger_opened": open
    })}
    onClick={onClick}
  >
    <span></span>
  </button>
);

export default MenuBurger;
