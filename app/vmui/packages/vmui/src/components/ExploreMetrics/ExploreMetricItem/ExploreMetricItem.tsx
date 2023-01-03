import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import ExploreMetricItemGraph from "../ExploreMetricGraph/ExploreMetricItemGraph";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import { MouseEvent } from "react";

interface ExploreMetricItemProps {
  name: string,
  job: string,
  instance: string
}

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({
  name,
  job,
  instance,
}) => {
  const isCounter = useMemo(() => /_sum?|_total?|_count?/.test(name), [name]);
  const isBucket = useMemo(() => /_bucket?/.test(name), [name]);

  const [rateEnabled, setRateEnabled] = useState(isCounter);

  const handleClickRate = (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
  };

  useEffect(() => {
    setRateEnabled(isCounter);
  }, [job]);

  return (
    <div className="vm-explore-metrics-item vm-block vm-block_empty-padding">
      <div className="vm-explore-metrics-item-header">
        <div className="vm-explore-metrics-item-header__name">{name}</div>
        {!isBucket && (
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
      <ExploreMetricItemGraph
        key={`${name}_${job}_${instance}_${rateEnabled}`}
        name={name}
        job={job}
        instance={instance}
        rateEnabled={rateEnabled}
        isBucket={isBucket}
      />
    </div>
  );
};

export default ExploreMetricItem;
