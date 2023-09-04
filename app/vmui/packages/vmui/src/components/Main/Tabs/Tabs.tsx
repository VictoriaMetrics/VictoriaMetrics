import React, { Component, FC, useRef, useState } from "preact/compat";
import { ReactNode, useEffect } from "react";
import { getCssVariable } from "../../../utils/theme";
import TabItem from "./TabItem";
import "./style.scss";
import useWindowSize from "../../../hooks/useWindowSize";

export interface TabItemType {
  value: string
  label?: string
  icon?: ReactNode
  className?: string
}

interface TabsProps {
  activeItem: string
  items: TabItemType[]
  color?: string
  onChange?: (value: string) => void
  indicatorPlacement?: "bottom" | "top"
  isNavLink?: boolean
}

const Tabs: FC<TabsProps> = ({
  activeItem,
  items,
  color = getCssVariable("color-primary"),
  onChange,
  indicatorPlacement = "bottom",
  isNavLink,
}) => {
  const windowSize = useWindowSize();
  const activeNavRef = useRef<Component>(null);
  const [indicatorPosition, setIndicatorPosition] = useState({ left: 0, width: 0, bottom: 0 });

  useEffect(() => {
    if(activeNavRef.current?.base instanceof HTMLElement) {
      const { offsetLeft: left, offsetWidth: width, offsetHeight: height } = activeNavRef.current.base;
      const positionTop = indicatorPlacement === "top";
      setIndicatorPosition({ left, width, bottom: positionTop ? height - 2 : 0 });
    }
  }, [windowSize, activeItem, activeNavRef, items]);

  return <div className="vm-tabs">
    {items.map(item => (
      <TabItem
        key={item.value}
        activeItem={activeItem}
        item={item}
        onChange={onChange}
        color={color}
        activeNavRef={activeNavRef}
        isNavLink={isNavLink}
      />
    ))}
    <div
      className="vm-tabs__indicator"
      style={{ ...indicatorPosition, borderColor: color }}
    />
  </div>;
};

export default Tabs;
