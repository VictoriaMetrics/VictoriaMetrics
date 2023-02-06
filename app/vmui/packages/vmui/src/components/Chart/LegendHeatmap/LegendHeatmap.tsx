import React, { FC, useEffect, useState } from "preact/compat";
import { gradMetal16 } from "../../../utils/uplot/heatmap";
import "./style.scss";

interface LegendHeatmapProps {
  min: number
  max: number
  value?: number
}

const LegendHeatmap: FC<LegendHeatmapProps> = ({ min, max, value }) => {

  const [hasValue, setHasValue] = useState(false);
  const [percent, setPercent] = useState(0);

  useEffect(() => {
    const valueIsNumber = typeof value === "number";
    setPercent(valueIsNumber ? (value - min) / (max - min) * 100 : 0);
    setHasValue(valueIsNumber);
  }, [value]);

  return (
    <div className="vm-legend-heatmap">
      <div
        className="vm-legend-heatmap-gradient"
        style={{ background: `linear-gradient(to right, ${gradMetal16.join(", ")})` }}
      >
        {hasValue && (
          <span
            className="vm-legend-heatmap-gradient__value"
            style={{ left: `${percent}%` }}
          >
            {value}
          </span>
        )}
      </div>
      <div className="vm-legend-heatmap__value">{min}</div>
      <div className="vm-legend-heatmap__value">{max}</div>
    </div>
  );
};

export default LegendHeatmap;
