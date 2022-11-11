import React, { FC, useRef, useState } from "preact/compat";
import { ReactNode, useEffect } from "react";
import "./style.scss";
import classNames from "classnames";
import { getVariableColor } from "../../../utils/theme";

interface TabsProps {
  activeItem: string
  items: {value: string, label: string, icon?: ReactNode, className?: string}[]
  color?: string
  onChange: (value: string) => void
}

const Tabs: FC<TabsProps> = ({
  activeItem,
  items,
  color = getVariableColor("primary"),
  onChange
}) => {
  const activeNavRef = useRef<HTMLDivElement>(null);
  const [indicatorPosition, setIndicatorPosition] = useState({ left: 0, width: 0, bottom: 0 });
  useEffect(() => {
    if(activeNavRef.current) {
      const { offsetLeft: left, offsetWidth: width } = activeNavRef.current;
      setIndicatorPosition({ left, width, bottom: 0 });
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
        onClick={() => onChange(item.value)}
      >
        {item.icon && <div className="vm-tabs-item__icon">{item.icon}</div>}
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
