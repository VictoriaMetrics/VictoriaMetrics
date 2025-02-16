import React, { FC, useMemo } from "react";
import { LegendItemType } from "../../../../types";
import { useLegendView } from "./hooks/useLegendView";
import LegendLines from "./LegendViews/LegendLines";
import LegendTable from "./LegendViews/LegendTable";
import { useHideDuplicateFields } from "./hooks/useHideDuplicateFields";

export type LegendProps = {
  labels: LegendItemType[];
  isAnomalyView?: boolean;
  duplicateFields?: string[];
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const LegendGroup: FC<LegendProps> = ({ labels, isAnomalyView, onChange }) => {
  const { isTableView } = useLegendView();
  const { duplicateFields } = useHideDuplicateFields(labels);

  const sortedLabels = useMemo(() => {
    return labels.sort((x, y) => (y.median || 0) - (x.median || 0));
  }, [labels]);

  if (isTableView) {
    return (
      <LegendTable
        labels={sortedLabels}
        isAnomalyView={isAnomalyView}
        duplicateFields={duplicateFields}
        onChange={onChange}
      />
    );
  }

  return (
    <LegendLines
      labels={sortedLabels}
      isAnomalyView={isAnomalyView}
      duplicateFields={duplicateFields}
      onChange={onChange}
    />
  );
};

export default LegendGroup;
