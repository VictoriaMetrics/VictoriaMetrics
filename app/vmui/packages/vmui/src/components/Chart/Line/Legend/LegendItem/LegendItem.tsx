import React, { FC, useMemo } from "preact/compat";
import { MouseEvent } from "react";
import { LegendItemType } from "../../../../../utils/uplot/types";
import "./style.scss";
import classNames from "classnames";
import { getFreeFields } from "./helpers";
import useCopyToClipboard from "../../../../../hooks/useCopyToClipboard";

interface LegendItemProps {
  legend: LegendItemType;
  onChange?: (item: LegendItemType, metaKey: boolean) => void;
  isHeatmap?: boolean;
}

const LegendItem: FC<LegendItemProps> = ({ legend, onChange, isHeatmap }) => {
  const copyToClipboard = useCopyToClipboard();

  const freeFormFields = useMemo(() => {
    const result = getFreeFields(legend);
    return isHeatmap ? result.filter(f => f.key !== "vmrange") : result;
  }, [legend, isHeatmap]);

  const calculations = legend.calculations;
  const showCalculations = Object.values(calculations).some(v => v);

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
      {!isHeatmap && (
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
      {!isHeatmap && showCalculations && (
        <div className="vm-legend-item-values">
          median:{calculations.median}, min:{calculations.min}, max:{calculations.max}, last:{calculations.last}
        </div>
      )}
    </div>
  );
};

export default LegendItem;
