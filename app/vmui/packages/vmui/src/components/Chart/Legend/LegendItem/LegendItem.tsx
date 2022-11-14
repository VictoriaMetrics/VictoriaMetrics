import React, { FC, useState, useMemo } from "preact/compat";
import { MouseEvent } from "react";
import { LegendItemType } from "../../../../utils/uplot/types";
import { getLegendLabel } from "../../../../utils/uplot/helpers";
import "./style.scss";
import classNames from "classnames";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { getFreeFields } from "./helpers";

interface LegendItemProps {
  legend: LegendItemType;
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const LegendItem: FC<LegendItemProps> = ({ legend, onChange }) => {
  const [copiedValue, setCopiedValue] = useState("");
  const freeFormFields = useMemo(() => getFreeFields(legend), [legend]);

  const handleClickFreeField = async (val: string, id: string) => {
    await navigator.clipboard.writeText(val);
    setCopiedValue(id);
    setTimeout(() => setCopiedValue(""), 2000);
  };

  const createHandlerClick = (legend: LegendItemType) => (e: MouseEvent<HTMLDivElement>) => {
    onChange(legend, e.ctrlKey || e.metaKey);
  };

  const createHandlerCopy = (freeField: string, id: string) => (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    handleClickFreeField(freeField, id);
  };


  return (
    <div
      className={classNames({
        "vm-legend-item": true,
        "vm-legend-item_hide": !legend.checked,
      })}
      onClick={createHandlerClick(legend)}
    >
      <div
        className="vm-legend-item__marker"
        style={{ backgroundColor: legend.color }}
      />
      <div className="vm-legend-item-info">
        <span className="vm-legend-item-info__label">
          {getLegendLabel(legend.label)}
        </span>

        &#160;&#123;
        {freeFormFields.map(f => (
          <Tooltip
            key={f.id}
            open={copiedValue === f.id}
            title={"Copied!"}
          >
            <span
              className="vm-legend-item-info__free-fields"
              key={f.key}
              onClick={createHandlerCopy(f.freeField, f.id)}
            >
              {f.freeField}
            </span>
          </Tooltip>
        ))}
        &#125;
      </div>
    </div>
  );
};

export default LegendItem;
