import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import ExploreMetricItemGraph from "../ExploreMetricGraph/ExploreMetricItemGraph";
import ExploreMetricItemHeader from "../ExploreMetricItemHeader/ExploreMetricItemHeader";
import "./style.scss";
import { GraphSize } from "../../../types";
import useWindowSize from "../../../hooks/useWindowSize";

interface ExploreMetricItemProps {
  name: string
  job: string
  instance: string
  index: number
  length: number
  size: GraphSize
  onRemoveItem: (name: string) => void
  onChangeOrder: (name: string, oldIndex: number, newIndex: number) => void
}

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({
  name,
  job,
  instance,
  index,
  length,
  size,
  onRemoveItem,
  onChangeOrder,
}) => {

  const isCounter = useMemo(() => /_sum?|_total?|_count?/.test(name), [name]);
  const isBucket = useMemo(() => /_bucket?/.test(name), [name]);

  const [rateEnabled, setRateEnabled] = useState(isCounter);

  const windowSize = useWindowSize();
  const graphHeight = useMemo(size.height, [size, windowSize]);

  useEffect(() => {
    setRateEnabled(isCounter);
  }, [job]);

  return (
    <div className="vm-explore-metrics-item vm-block vm-block_empty-padding">
      <ExploreMetricItemHeader
        name={name}
        index={index}
        length={length}
        isBucket={isBucket}
        rateEnabled={rateEnabled}
        size={size.id}
        onChangeRate={setRateEnabled}
        onRemoveItem={onRemoveItem}
        onChangeOrder={onChangeOrder}
      />
      <ExploreMetricItemGraph
        key={`${name}_${job}_${instance}_${rateEnabled}`}
        name={name}
        job={job}
        instance={instance}
        rateEnabled={rateEnabled}
        isBucket={isBucket}
        height={graphHeight}
      />
    </div>
  );
};

export default ExploreMetricItem;
