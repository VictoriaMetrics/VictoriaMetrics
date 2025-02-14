import React, { FC } from "preact/compat";
import LegendItem from "../LegendItem/LegendItem";
import { LegendProps } from "../LegendGroup";

const LegendLines: FC<LegendProps> = ({ labels, isAnomalyView, duplicateFields, onChange }) => {

  return (
    <div className="vm-legend-item-container">
      {labels.map((legendItem) =>
        <LegendItem
          key={legendItem.label}
          legend={legendItem}
          isAnomalyView={isAnomalyView}
          duplicateFields={duplicateFields}
          onChange={onChange}
        />
      )}
    </div>
  );
};

export default LegendLines;
