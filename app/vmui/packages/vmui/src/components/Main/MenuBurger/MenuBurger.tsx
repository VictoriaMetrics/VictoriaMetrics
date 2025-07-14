import classNames from "classnames";
import "./style.scss";

const MenuBurger = ({ open }: {open: boolean}) => (
  <button
    className={classNames({
      "vm-menu-burger": true,
      "vm-menu-burger_opened": open
    })}
    aria-label="menu"
  >
    <span></span>
  </button>
);

export default MenuBurger;
