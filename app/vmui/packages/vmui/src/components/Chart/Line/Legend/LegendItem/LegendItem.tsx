import React, { FC, useMemo } from "preact/compat";
import { MouseEvent } from "react";
import { LegendItemType } from "../../../../../types";
import "./style.scss";
import classNames from "classnames";
import { getFreeFields } from "./helpers";
import useCopyToClipboard from "../../../../../hooks/useCopyToClipboard";
import { STATS_ORDER } from "../../../../../constants/graph";
import { useShowStats } from "../hooks/useShowStats";
import { useLegendFormat } from "../hooks/useLegendFormat";
import { getLabelAlias } from "../../../../../utils/metric";

interface LegendItemProps {
  legend: LegendItemType;
  onChange?: (item: LegendItemType, metaKey: boolean) => void;
  isAnomalyView?: boolean;
  duplicateFields?: string[];
}

const LegendItem: FC<LegendItemProps> = ({ legend, onChange, duplicateFields, isAnomalyView }) => {
  const copyToClipboard = useCopyToClipboard();
  const { hideStats } = useShowStats();

  const { format } = useLegendFormat();
  const formattedLabel = getLabelAlias(legend.freeFormFields, format);

  const freeFormFields = useMemo(() => {
    const result = getFreeFields(legend);
    return duplicateFields?.length
      ? result.filter(f => !duplicateFields.includes(f.key))
      : result;
  }, [legend, duplicateFields]);

  const statsFormatted = legend.statsFormatted;
  const showStats = Object.values(statsFormatted).some(v => v);

  const createHandlerClick = (legend: LegendItemType) => (e: MouseEvent<HTMLDivElement>) => {
    onChange && onChange(legend, e.ctrlKey || e.metaKey);
  };

  const createHandlerCopy = (freeField: string) => async (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    await copyToClipboard(freeField, `${freeField} has been copied`);
  };

  return (
    <div
      className={classNames({
        "vm-legend-item": true,
        "vm-legend-row": true,
        "vm-legend-item_hide": !legend.checked,
      })}
      onClick={createHandlerClick(legend)}
    >
      {!isAnomalyView && (
        <div
          className="vm-legend-item__marker"
          style={{ backgroundColor: legend.color }}
        />
      )}
      <div className="vm-legend-item-info">
        <span className="vm-legend-item-info__label">
          {legend.hasAlias && legend.label}
          {!legend.hasAlias && format && formattedLabel}
          {!legend.hasAlias && !format && (
            <>
              {legend.freeFormFields["__name__"]}
              {!!freeFormFields.length && <> &#123;</>}
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
            </>
          )}
        </span>
      </div>
      {!hideStats && showStats && (
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
