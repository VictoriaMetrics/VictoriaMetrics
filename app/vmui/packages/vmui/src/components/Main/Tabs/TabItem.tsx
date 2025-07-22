import { Component, FC, Ref } from "preact/compat";
import classNames from "classnames";
import { getCssVariable } from "../../../utils/theme";
import { TabItemType } from "./Tabs";
import TabItemWrapper from "./TabItemWrapper";
import "./style.scss";

interface TabItemProps {
  activeItem: string
  item: TabItemType
  color?: string
  onChange?: (value: string) => void
  activeNavRef: Ref<Component>
  isNavLink?: boolean
}

const TabItem: FC<TabItemProps> = ({
  activeItem,
  item,
  color = getCssVariable("color-primary"),
  activeNavRef,
  onChange,
  isNavLink
}) => {
  const createHandlerClickTab = (value: string) => () => {
    onChange && onChange(value);
  };

  return (
    <TabItemWrapper
      className={classNames({
        "vm-tabs-item": true,
        "vm-tabs-item_active": activeItem === item.value,
        [item.className || ""]: item.className
      })}
      isNavLink={isNavLink}
      to={item.value}
      style={{ color: color }}
      onClick={createHandlerClickTab(item.value)}
      ref={activeItem === item.value ? activeNavRef : undefined}
    >
      {item.icon && (
        <div
          className={classNames({
            "vm-tabs-item__icon": true,
            "vm-tabs-item__icon_single": !item.label
          })}
        >
          {item.icon}
        </div>
      )}
      {item.label}
    </TabItemWrapper>
  );
};

export default TabItem;
