import "./style.scss";
import { ReactNode } from "react";

export type BadgeColor = "firing" | "inactive" | "pending" | "nomatch" | "unhealthy" | "ok" | "passive";

interface BadgeItem {
  value?: number | string;
  color: BadgeColor;  
}

interface BadgesProps {
  items: Record<string, BadgeItem>;
  align?: "center" | "start" | "end";
  children?: ReactNode;
}

const Badges = ({ items, children, align = "start" }: BadgesProps) => {
  return (
    <div
      className="vm-badges"
      style={{ "justify-content": align }}
    >
      {Object.entries(items).map(([name, props]) => (
        <span
          key={name}
          className={`vm-badge ${props.color}`}
        >{props.value ? `${name}: ${props.value}` : name}</span>
      ))}
      {children}
    </div>
  );
};

export default Badges;
