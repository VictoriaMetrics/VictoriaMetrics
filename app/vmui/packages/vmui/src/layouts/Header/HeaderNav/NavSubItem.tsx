import { FC, useRef, useState, Dispatch, SetStateAction } from "preact/compat";
import classNames from "classnames";
import { ArrowDropDownIcon } from "../../../components/Main/Icons";
import Popper from "../../../components/Main/Popper/Popper";
import NavItem from "./NavItem";
import { NavigationItem, NavigationItemType } from "../../../router/navigation";

interface NavItemProps {
  activeMenu: string,
  label: string,
  submenu: NavigationItem[],
  color?: string
  background?: string
  direction?: "row" | "column"
  openMenu: string | null,
  setOpenMenu: Dispatch<SetStateAction<string | null>>,
}

const NavSubItem: FC<NavItemProps> = ({
  activeMenu,
  label,
  color,
  background,
  submenu,
  direction = "row",
  openMenu,
  setOpenMenu,
}) => {
  const [menuTimeout, setMenuTimeout] = useState<NodeJS.Timeout | null>(null);
  const buttonRef = useRef<HTMLDivElement>(null);

  const openSubmenu = openMenu === label;
  const handleCloseSubmenu = () => setOpenMenu(prev => (prev === label ? null : prev));

  const handleOpenSubmenu = () => {
    if (direction === "row" || !openSubmenu) setOpenMenu(label);
    if (direction === "column" && openSubmenu) handleCloseSubmenu();
    if (direction === "row" && menuTimeout) clearTimeout(menuTimeout);
  };

  const handleMouseLeave = () => {
    if (menuTimeout) clearTimeout(menuTimeout);
    const timeout = setTimeout(handleCloseSubmenu, 300);
    setMenuTimeout(timeout);
  };

  const handleMouseEnterPopup = () => {
    if (menuTimeout) clearTimeout(menuTimeout);
  };

  return (
    <div
      className={classNames({
        "vm-header-nav-item_open": openSubmenu,
      })}
      style={{ color }}
      onMouseEnter={direction === "column" ? undefined : handleOpenSubmenu}
      onMouseLeave={direction === "column" ? undefined : handleMouseLeave}
      onClick={direction === "column" ? handleOpenSubmenu : undefined}
      ref={buttonRef}
    >
      <div
        className={classNames({
          "vm-header-nav-item": true,
          "vm-header-nav-item_sub": true,
          "vm-header-nav-item_active": submenu.find(m => m.value === activeMenu),
        })}
      >
        {label}
        <ArrowDropDownIcon/>
      </div>
      {direction === "column" ? (
        <div
          className="vm-header-nav-item-submenu"
          style={{ background }}
        >
          {submenu.map(sm => (
            <NavItem
              key={sm.value}
              activeMenu={activeMenu}
              value={sm.value || ""}
              label={sm.label || ""}
              color={color}
              type={sm.type || NavigationItemType.internalLink}
            />
          ))}
        </div>
      ) : (
        <Popper
          open={openSubmenu}
          placement="bottom-left"
          offset={{ top: 12, left: 0 }}
          onClose={handleCloseSubmenu}
          buttonRef={buttonRef}
        >
          <div
            className="vm-header-nav-item-submenu"
            style={{ background }}
            onMouseLeave={handleMouseLeave}
            onMouseEnter={handleMouseEnterPopup}
          >
            {submenu.map(sm => (
              <NavItem
                key={sm.value}
                activeMenu={activeMenu}
                value={sm.value || ""}
                label={sm.label || ""}
                color={color}
                type={sm.type || NavigationItemType.internalLink}
              />
            ))}
          </div>
        </Popper>
      )}
    </div>
  );
};

export default NavSubItem;
