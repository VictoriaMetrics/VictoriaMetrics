import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import ExploreMetricItemGraph from "../ExploreMetricGraph/ExploreMetricItemGraph";
import ExploreMetricItemHeader from "../ExploreMetricItemHeader/ExploreMetricItemHeader";
import "./style.scss";
import useResize from "../../../hooks/useResize";

interface ExploreMetricItemProps {
  name: string
  job: string
  instance: string
  index: number
  onRemoveItem: (name: string) => void
  onChangeOrder: (name: string, oldIndex: number, newIndex: number) => void
}

export const sizeVariants = [
  {
    id: "small",
    height: () => window.innerHeight * 0.2
  },
  {
    id: "medium",
    isDefault: true,
    height: () => window.innerHeight * 0.4
  },
  {
    id: "large",
    height: () => window.innerHeight * 0.8
  },
];

const ExploreMetricItem: FC<ExploreMetricItemProps> = ({
  name,
  job,
  instance,
  index,
  onRemoveItem,
  onChangeOrder,
}) => {

  const isCounter = useMemo(() => /_sum?|_total?|_count?/.test(name), [name]);
  const isBucket = useMemo(() => /_bucket?/.test(name), [name]);

  const [rateEnabled, setRateEnabled] = useState(isCounter);
  const [showLegend, setShowLegend] = useState(false);
  const [size, setSize] = useState(sizeVariants.find(v => v.isDefault) || sizeVariants[0]);

  const windowSize = useResize(document.body);
  const graphHeight = useMemo(size.height, [size, windowSize]);

  const handleChangeSize = (id: string) => {
    const target = sizeVariants.find(variant => variant.id === id);
    if (target) setSize(target);
  };

  useEffect(() => {
    setRateEnabled(isCounter);
  }, [job]);

  return (
    <div className="vm-explore-metrics-item vm-block vm-block_empty-padding">
      <ExploreMetricItemHeader
        name={name}
        index={index}
        isBucket={isBucket}
        rateEnabled={rateEnabled}
        showLegend={showLegend}
        size={size.id}
        onChangeRate={setRateEnabled}
        onChangeLegend={setShowLegend}
        onRemoveItem={onRemoveItem}
        onChangeOrder={onChangeOrder}
        onChangeSize={handleChangeSize}
      />
      <ExploreMetricItemGraph
        key={`${name}_${job}_${instance}_${rateEnabled}`}
        name={name}
        job={job}
        instance={instance}
        rateEnabled={rateEnabled}
        isBucket={isBucket}
        showLegend={showLegend}
        height={graphHeight}
      />
    </div>
  );
};

export default ExploreMetricItem;
