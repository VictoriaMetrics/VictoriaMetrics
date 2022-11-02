import React, { FC, useMemo, useState } from "preact/compat";
import { LegendItemType } from "../../../utils/uplot/types";
import LegendItem from "./LegendItem/LegendItem";
import "./legend.css";

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
    <div className="legendWrapper">
      {groups.map((group) => <div
        className="legendGroup"
        key={group}
      >
        <div className="legendGroupTitle">
          <span className="legendGroupQuery">Query {group}</span>
          <span>(&quot;{query[group - 1]}&quot;)</span>
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
