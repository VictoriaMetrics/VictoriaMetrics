import React, { FC } from "preact/compat";
import Accordion from "../../../components/Main/Accordion/Accordion";
import ExploreMetricItemGraph from "./ExploreMetricItemGraph";
import "./style.scss";

interface ExploreMetricItemProps {
  name: string,
  job: string,
  instance: string
}

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({ name, job, instance }) => {

  return (
    <div className="vm-explore-metrics-item">
      <Accordion
        title={<div className="vm-explore-metrics-item__header">{name}</div>}
      >
        <ExploreMetricItemGraph
          name={name}
          job={job}
          instance={instance}
        />
      </Accordion>
    </div>
  );
};

export default ExploreMetricItem;
