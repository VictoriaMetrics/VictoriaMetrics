import React, { FC, useEffect, useState } from "preact/compat";
import { gradMetal16 } from "../../../utils/uplot/heatmap";
import "./style.scss";

interface LegendHeatmapProps {
  min: number
  max: number
  value?: number
}

const LegendHeatmap: FC<LegendHeatmapProps> = ({ min, max, value }) => {

  const [percent, setPercent] = useState(0);
  const [valueFormat, setValueFormat] = useState("");
  const [minFormat, setMinFormat] = useState("");
  const [maxFormat, setMaxFormat] = useState("");

  useEffect(() => {
    setPercent(value ? (value - min) / (max - min) * 100 : 0);
    setValueFormat(value ? `${value}%` : "");
    setMinFormat(`${min}%`);
    setMaxFormat(`${max}%`);
  }, [value, min, max]);

  return (
    <div className="vm-legend-heatmap">
      <div
        className="vm-legend-heatmap-gradient"
        style={{ background: `linear-gradient(to right, ${gradMetal16.join(", ")})` }}
      >
        {!!value && (
          <div
            className="vm-legend-heatmap-gradient__value"
            style={{ left: `${percent}%` }}
          >
            <span>{valueFormat}</span>
          </div>
        )}
      </div>
      <div className="vm-legend-heatmap__value">{minFormat}</div>
      <div className="vm-legend-heatmap__value">{maxFormat}</div>
    </div>
  );
};

export default LegendHeatmap;
