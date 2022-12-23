import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import Accordion from "../../../components/Main/Accordion/Accordion";
import ExploreMetricItemGraph from "./ExploreMetricItemGraph";
import "./style.scss";
import Switch from "../../../components/Main/Switch/Switch";
import { MouseEvent } from "react";

interface ExploreMetricItemProps {
  name: string,
  job: string,
  instance: string
  openMetrics: string[]
  onOpen: (val: boolean, id: string) => void
}

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({
  name,
  job,
  instance,
  openMetrics,
  onOpen
}) => {
  const expanded = useMemo(() => openMetrics.includes(name), [name, openMetrics]);
  const isCounter = useMemo(() => /_sum?|_total?|_count?/.test(name), [name]);
  const isBucket = useMemo(() => /_bucket?/.test(name), [name]);

  const [rateEnabled, setRateEnabled] = useState(isCounter);

  const handleOpenAccordion = (val: boolean) => {
    onOpen(val, name);
  };

  const handleClickRate = (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
  };

  useEffect(() => {
    setRateEnabled(isCounter);
  }, [job, expanded]);

  const Title = () => (
    <div className="vm-explore-metrics-item-header">
      <div className="vm-explore-metrics-item-header__name">{name}</div>
      {expanded && !isBucket && (
        <div
          className="vm-explore-metrics-item-header__rate"
          onClick={handleClickRate}
        >
          <Switch
            label={<span>rate()</span>}
            value={rateEnabled}
            onChange={setRateEnabled}
          />
        </div>
      )}
    </div>
  );

  return (
    <div className="vm-explore-metrics-item">
      <Accordion
        title={<Title/>}
        defaultExpanded={expanded}
        onChange={handleOpenAccordion}
      >
        <ExploreMetricItemGraph
          key={`${name}_${job}_${instance}_${rateEnabled}`}
          name={name}
          job={job}
          instance={instance}
          rateEnabled={rateEnabled}
          isBucket={isBucket}
        />
      </Accordion>
    </div>
  );
};

export default ExploreMetricItem;
