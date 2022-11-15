import React, { FC, useRef, useState } from "preact/compat";
import { ReactNode, useEffect } from "react";
import "./style.scss";
import classNames from "classnames";
import { getCssVariable } from "../../../utils/theme";

interface TabsProps {
  activeItem: string
  items: {value: string, label?: string, icon?: ReactNode, className?: string}[]
  color?: string
  onChange: (value: string) => void
  indicatorPlacement?: "bottom" | "top"
}

const Tabs: FC<TabsProps> = ({
  activeItem,
  items,
  color = getCssVariable("color-primary"),
  onChange,
  indicatorPlacement = "bottom"
}) => {
  const activeNavRef = useRef<HTMLDivElement>(null);
  const [indicatorPosition, setIndicatorPosition] = useState({ left: 0, width: 0, bottom: 0 });

  const createHandlerClickTab = (value: string) => () => {
    onChange(value);
  };

  useEffect(() => {
    if(activeNavRef.current) {
      const { offsetLeft: left, offsetWidth: width, offsetHeight: height } = activeNavRef.current;
      const positionTop = indicatorPlacement === "top";
      setIndicatorPosition({ left, width, bottom: positionTop ? height - 2 : 0 });
    }
  }, [activeItem, activeNavRef, items]);

  return <div className="vm-tabs">
    {items.map(item => (
      <div
        className={classNames({
          "vm-tabs-item": true,
          "vm-tabs-item_active": activeItem === item.value,
          [item.className || ""]: item.className
        })}
        ref={activeItem === item.value ? activeNavRef : undefined}
        key={item.value}
        style={{ color: color }}
        onClick={createHandlerClickTab(item.value)}
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
      </div>
    ))}
    <div
      className="vm-tabs__indicator"
      style={{ ...indicatorPosition, borderColor: color }}
    />
  </div>;
};

export default Tabs;
