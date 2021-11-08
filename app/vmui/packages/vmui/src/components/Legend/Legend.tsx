import React, {FC} from "react";
import {hexToRGB} from "../../utils/color";
import "./legend.css";

export interface LegendItem {
  label: string;
  color: string;
  checked: boolean;
}

export interface LegendProps {
  labels: LegendItem[];
  onChange: (legend: string, metaKey: boolean) => void;
}

export const Legend: FC<LegendProps> = ({labels, onChange}) => {
  return <div className="legendWrapper">
    {labels.map((legendItem: LegendItem) =>
      <div className={legendItem.checked ? "legendItem" : "legendItem legendItemHide"}
        key={legendItem.label}
        onClick={(e) => onChange(legendItem.label, e.ctrlKey || e.metaKey)}>
        <div className="legendMarker"
          style={{
            borderColor: legendItem.color,
            backgroundColor: `rgba(${hexToRGB(legendItem.color)}, 0.1)`
          }}/>
        <div className="legendLabel">{legendItem.checked} {legendItem.label}</div>
      </div>
    )}
  </div>;
};