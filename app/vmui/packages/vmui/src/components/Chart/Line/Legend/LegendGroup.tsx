import React, { FC, MouseEvent, useMemo } from "react";
import { LegendItemType } from "../../../../types";
import { useLegendView } from "./hooks/useLegendView";
import LegendLines from "./LegendViews/LegendLines";
import LegendTable from "./LegendViews/LegendTable";
import { useHideDuplicateFields } from "./hooks/useHideDuplicateFields";
import Accordion from "../../../Main/Accordion/Accordion";
import { useLegendGroup } from "./hooks/useLegendGroup";
import useCopyToClipboard from "../../../../hooks/useCopyToClipboard";

export type LegendProps = {
  labels: LegendItemType[];
  isAnomalyView?: boolean;
  duplicateFields?: string[];
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

interface LegendGroupProps extends LegendProps {
  group: string | number;
}

const LegendGroup: FC<LegendGroupProps> = ({ labels, group, isAnomalyView, onChange }) => {
  const { isTableView } = useLegendView();
  const { groupByLabel } = useLegendGroup();
  const copyToClipboard = useCopyToClipboard();
  const { duplicateFields } = useHideDuplicateFields(labels);

  const sortedLabels = useMemo(() => {
    return labels.sort((x, y) => (y.median || 0) - (x.median || 0));
  }, [labels]);

  const createHandlerCopy = (value: string) => async (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    await copyToClipboard(value, `${value} has been copied`);
  };

  const Content = isTableView ? LegendTable : LegendLines;

  return (
    <div
      className="vm-legend-group"
      key={group}
    >
      <Accordion
        defaultExpanded={true}
        title={(
          <div className="vm-legend-group-header">
            <div className="vm-legend-group-header-title">
              Group by{groupByLabel ? "" : " query"}: <b>{group}</b>
            </div>
            {!!duplicateFields.length && (
              <div className="vm-legend-group-header-labels">
                common labels:
                &#123;
                {duplicateFields.map(label => (
                  <div
                    key={label}
                    onClick={createHandlerCopy(`${label}="${labels[0].freeFormFields[label]}"`)}
                    className="vm-legend-group-header-labels__item"
                  >
                    {`${label}="${labels[0].freeFormFields[label]}"`}
                  </div>
                ))}
                &#125;
              </div>
            )}
          </div>
        )}
      >
        <Content
          labels={sortedLabels}
          isAnomalyView={isAnomalyView}
          duplicateFields={duplicateFields}
          onChange={onChange}
        />
      </Accordion>
    </div>
  );
};

export default LegendGroup;
