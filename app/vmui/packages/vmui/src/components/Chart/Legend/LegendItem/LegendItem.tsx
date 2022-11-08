import React, { FC, useState, useMemo } from "preact/compat";
import { LegendItemType } from "../../../../utils/uplot/types";
import { getLegendLabel } from "../../../../utils/uplot/helpers";
import "./legendItem.css";

interface LegendItemProps {
  legend: LegendItemType;
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const LegendItem: FC<LegendItemProps> = ({ legend, onChange }) => {
  const freeFormFields = useMemo(() => {
    const keys = Object.keys(legend.freeFormFields).filter(f => f !== "__name__");
    return keys.map(f => {
      const freeField = `${f}="${legend.freeFormFields[f]}"`;
      const id = `${legend.label}.${freeField}`;
      return { id, key: f, freeField, };
    });
  }, [legend]);

  const [copiedValue, setCopiedValue] = useState("");

  const handleClickFreeField = async (val: string, id: string) => {
    await navigator.clipboard.writeText(val);
    setCopiedValue(id);
    setTimeout(() => setCopiedValue(""), 2000);
  };

  return (
    <div
      className={legend.checked ? "legendItem" : "legendItem legendItemHide"}
      onClick={(e) => onChange(legend, e.ctrlKey || e.metaKey)}
    >
      <div
        className="legendMarker"
        style={{ backgroundColor: legend.color }}
      />
      <div className="legendLabel">
        <span>
          {getLegendLabel(legend.label)}
        </span>

        &#160;&#123;

        {/*<Tooltip*/}
        {/* arrow*/}
        {/*  key={f.id}*/}
        {/*  open={copiedValue === f.id}*/}
        {/*  title={"Copied!"}*/}
        {/*>*/}
        {freeFormFields.map(f => (
          <span
            key={f.key}
            className="legendFreeFields"
            onClick={(e) => {
              e.stopPropagation();
              handleClickFreeField(f.freeField, f.id);
            }}
          >
            {f.key}: {legend.freeFormFields[f.key]}
          </span>))}
        {/*</Tooltip>*/}
        &#125;
      </div>
    </div>
  );
};

export default LegendItem;
