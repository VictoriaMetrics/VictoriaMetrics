import React, { FC, useMemo } from "preact/compat";
import { LegendItemType } from "../../../../types";
import LegendItem from "./LegendItem/LegendItem";
import Accordion from "../../../Main/Accordion/Accordion";
import "./style.scss";

interface LegendProps {
  labels: LegendItemType[];
  query: string[];
  isAnomalyView?: boolean;
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const Legend: FC<LegendProps> = ({ labels, query, isAnomalyView, onChange }) => {
  const groups = useMemo(() => {
    return Array.from(new Set(labels.map(l => l.group)));
  }, [labels]);
  const showQueryNum = groups.length > 1;

  return <>
    <div className="vm-legend">
      {groups.map((group) => (
        <div
          className="vm-legend-group"
          key={group}
        >
          <Accordion
            defaultExpanded={true}
            title={(
              <div className="vm-legend-group-title">
                {showQueryNum && (
                  <span className="vm-legend-group-title__count">Query {group}: </span>
                )}
                <span className="vm-legend-group-title__query">{query[group - 1]}</span>
              </div>
            )}
          >
            <div>
              {labels.filter(l => l.group === group).sort((x, y) => (y.median || 0) - (x.median || 0)).map((legendItem: LegendItemType) =>
                <LegendItem
                  key={legendItem.label}
                  legend={legendItem}
                  isAnomalyView={isAnomalyView}
                  onChange={onChange}
                />
              )}
            </div>
          </Accordion>
        </div>
      ))}
    </div>
  </>;
};

export default Legend;
