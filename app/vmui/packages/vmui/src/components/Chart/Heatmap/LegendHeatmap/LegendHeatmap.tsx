import { FC, useEffect, useMemo, useState } from "preact/compat";
import { gradMetal16 } from "../../../../utils/uplot";
import "./style.scss";
import { ChartTooltipProps } from "../../ChartTooltip/ChartTooltip";

interface LegendHeatmapProps {
  min: number
  max: number
  legendValue: ChartTooltipProps | null,
}

const LegendHeatmap: FC<LegendHeatmapProps> = ({
  min,
  max,
  legendValue,
}) => {

  const [percent, setPercent] = useState(0);
  const [valueFormat, setValueFormat] = useState("");
  const [minFormat, setMinFormat] = useState("");
  const [maxFormat, setMaxFormat] = useState("");

  const value = useMemo(() => {
    const n = Number(String(legendValue?.value ?? "").replace("%","").replace(",", "."));
    return Number.isFinite(n) ? n : 0;
  }, [legendValue?.value]);

  useEffect(() => {
    setPercent(value ? (value - min) / (max - min) * 100 : 0);
    setValueFormat(value ? `${value}%` : "");
    setMinFormat(`${min}%`);
    setMaxFormat(`${max}%`);
  }, [value, min, max]);

  return (
    <div className="vm-legend-heatmap__wrapper">
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
    </div>
  );
};

export default LegendHeatmap;
