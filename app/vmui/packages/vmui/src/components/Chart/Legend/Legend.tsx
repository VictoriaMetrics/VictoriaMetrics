import React, { FC, useMemo } from "preact/compat";
import { LegendItemType } from "../../../utils/uplot/types";
import LegendItem from "./LegendItem/LegendItem";
import "./style.scss";

interface LegendProps {
  labels: LegendItemType[];
  query: string[];
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const Legend: FC<LegendProps> = ({ labels, query, onChange }) => {
  const groups = useMemo(() => {
    return Array.from(new Set(labels.map(l => l.group)));
  }, [labels]);

  return <>
    <div className="vm-legend">
      {groups.map((group) => <div
        className="vm-legend-group"
        key={group}
      >
        <div className="vm-legend-group-title">
          <span className="vm-legend-group-title__count">Query {group}: </span>
          <span className="vm-legend-group-title__query">{query[group - 1]}</span>
        </div>
        <div>
          {labels.filter(l => l.group === group).map((legendItem: LegendItemType) =>
            <LegendItem
              key={legendItem.label}
              legend={legendItem}
              onChange={onChange}
            />
          )}
        </div>
      </div>)}
    </div>
  </>;
};

export default Legend;
