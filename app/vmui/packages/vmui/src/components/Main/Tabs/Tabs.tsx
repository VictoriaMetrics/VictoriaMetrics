import React, { FC, useRef, useState } from "preact/compat";
import { ReactNode, useEffect } from "react";
import "./style.scss";
import classNames from "classnames";

interface TabsProps {
  activeItem: string
  items: {value: string, label: string, icon?: ReactNode}[]
  color: string
  onChange: (value: string) => void
}

const Tabs: FC<TabsProps> = ({ activeItem, items, color, onChange }) => {
  const activeNavRef = useRef<HTMLDivElement>(null);
  const [indicatorPosition, setIndicatorPosition] = useState({ left: 0, width: 0, top: 0 });
  useEffect(() => {
    if(activeNavRef.current) {
      const { left, width, top, height } = activeNavRef.current.getBoundingClientRect();
      setIndicatorPosition({ left, width, top: top + height });
    }
  }, [activeItem, activeNavRef]);

  return <div className="vm-tabs">
    {items.map(item => (
      <div
        className={classNames({
          "vm-tabs__item": true,
          "vm-tabs__item_active": activeItem === item.value
        })}
        ref={activeItem === item.value ? activeNavRef : undefined}
        key={item.value}
        style={{ color: color }}
        onClick={() => onChange(item.value)}
      >
        {item.icon && <div>{item.icon}</div>}
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
