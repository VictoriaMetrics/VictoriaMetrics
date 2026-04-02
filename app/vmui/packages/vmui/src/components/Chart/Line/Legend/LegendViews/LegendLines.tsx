import { FC } from "preact/compat";
import LegendItem from "../LegendItem/LegendItem";
import { LegendProps } from "../LegendGroup";

const LegendLines: FC<LegendProps> = ({ labels, duplicateFields, onChange }) => {

  return (
    <div className="vm-legend-item-container">
      {labels.map((legendItem) =>
        <LegendItem
          key={legendItem.label}
          legend={legendItem}
          duplicateFields={duplicateFields}
          onChange={onChange}
        />
      )}
    </div>
  );
};

export default LegendLines;
