import React, { FC, useMemo } from "preact/compat";
import { MouseEvent } from "react";
import { LegendItemType } from "../../../../../types";
import "./style.scss";
import classNames from "classnames";
import { getFreeFields } from "./helpers";
import useCopyToClipboard from "../../../../../hooks/useCopyToClipboard";
import { STATS_ORDER } from "../../../../../constants/graph";

interface LegendItemProps {
  legend: LegendItemType;
  onChange?: (item: LegendItemType, metaKey: boolean) => void;
  isHeatmap?: boolean;
  isAnomalyView?: boolean;
}

const LegendItem: FC<LegendItemProps> = ({ legend, onChange, isHeatmap, isAnomalyView }) => {
  const copyToClipboard = useCopyToClipboard();

  const freeFormFields = useMemo(() => {
    const result = getFreeFields(legend);
    return isHeatmap ? result.filter(f => f.key !== "vmrange") : result;
  }, [legend, isHeatmap]);

  const statsFormatted = legend.statsFormatted;
  const showStats = Object.values(statsFormatted).some(v => v);

  const handleClickFreeField = async (val: string) => {
    await copyToClipboard(val, `${val} has been copied`);
  };

  const createHandlerClick = (legend: LegendItemType) => (e: MouseEvent<HTMLDivElement>) => {
    onChange && onChange(legend, e.ctrlKey || e.metaKey);
  };

  const createHandlerCopy = (freeField: string) => (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    handleClickFreeField(freeField);
  };

  return (
    <div
      className={classNames({
        "vm-legend-item": true,
        "vm-legend-row": true,
        "vm-legend-item_hide": !legend.checked && !isHeatmap,
        "vm-legend-item_static": isHeatmap,
      })}
      onClick={createHandlerClick(legend)}
    >
      {!isAnomalyView && !isHeatmap && (
        <div
          className="vm-legend-item__marker"
          style={{ backgroundColor: legend.color }}
        />
      )}
      <div className="vm-legend-item-info">
        <span className="vm-legend-item-info__label">
          {legend.freeFormFields["__name__"]}
          {!!freeFormFields.length && <>&#123;</>}
          {freeFormFields.map((f, i) => (
            <span
              className="vm-legend-item-info__free-fields"
              key={f.key}
              onClick={createHandlerCopy(f.freeField)}
              title="copy to clipboard"
            >
              {f.freeField}{i + 1 < freeFormFields.length && ","}
            </span>
          ))}
          {!!freeFormFields.length && <>&#125;</>}
        </span>
      </div>
      {!isHeatmap && showStats && (
        <div className="vm-legend-item-stats">
          {STATS_ORDER.map((key, i) => (
            <div
              className="vm-legend-item-stats-row"
              key={i}
            >
              <span className="vm-legend-item-stats-row__key">{key}:</span>
              <span className="vm-legend-item-stats-row__value">{statsFormatted[key]}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default LegendItem;
