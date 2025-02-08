import React, { FC } from "preact/compat";
import { LegendItemType } from "../../../../types";
import Accordion from "../../../Main/Accordion/Accordion";
import "./style.scss";
import LegendGroup from "./LegendGroup";
import { useLegendGroup } from "./hooks/useLegendGroup";
import { useGroupSeries } from "./hooks/useGroupSeries";

export type QueryGroup = {
  group: number | string;
  items: LegendItemType[]
}

interface LegendProps {
  labels: LegendItemType[];
  query: string[];
  isAnomalyView?: boolean;
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const Legend: FC<LegendProps> = ({ labels, query, isAnomalyView, onChange }) => {
  const { groupByLabel } = useLegendGroup();
  const groupSeries = useGroupSeries({ labels, query, groupByLabel });

  return <>
    <div className="vm-legend">
      <div>
        {groupSeries.map(({ group, items }) => (
          <div
            className="vm-legend-group"
            key={group}
          >
            <Accordion
              defaultExpanded={true}
              title={(
                <div className="vm-legend-group-title">
                  Group by{groupByLabel ? "" : " query"}: <b>{group}</b>
                </div>
              )}
            >
              <LegendGroup
                labels={items}
                isAnomalyView={isAnomalyView}
                onChange={onChange}
              />
            </Accordion>
          </div>
        ))}
      </div>
    </div>
  </>;
};

export default Legend;
