import React, { FC } from "preact/compat";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";
import { ArrowDownIcon, CloseIcon } from "../../Main/Icons";

interface ExploreMetricItemControlsProps {
  name: string
  index: number
  isBucket: boolean
  rateEnabled: boolean
  size: string
  onChangeRate: (val: boolean) => void
  onRemoveItem: (name: string) => void
  onChangeOrder: (name: string, oldIndex: number, newIndex: number) => void
}

const ExploreMetricItemHeader: FC<ExploreMetricItemControlsProps> = ({
  name,
  index,
  isBucket,
  rateEnabled,
  onChangeRate,
  onRemoveItem,
  onChangeOrder,
}) => {

  const handleClickRemove = () => {
    onRemoveItem(name);
  };

  const handleOrderDown = () => {
    onChangeOrder(name, index, index + 1);
  };

  const handleOrderUp = () => {
    onChangeOrder(name, index, index - 1);
  };

  return (
    <div className="vm-explore-metrics-item-header">
      <div className="vm-explore-metrics-item-header-order">
        <Tooltip title="move graph up">
          <Button
            className="vm-explore-metrics-item-header-order__up"
            startIcon={<ArrowDownIcon/>}
            variant="text"
            color="gray"
            size="small"
            onClick={handleOrderUp}
          />
        </Tooltip>
        <div className="vm-explore-metrics-item-header__index">#{index+1}</div>
        <Tooltip title="move graph down">
          <Button
            className="vm-explore-metrics-item-header-order__down"
            startIcon={<ArrowDownIcon/>}
            variant="text"
            color="gray"
            size="small"
            onClick={handleOrderDown}
          />
        </Tooltip>
      </div>
      <div className="vm-explore-metrics-item-header__name">{name}</div>
      {!isBucket && (
        <div className="vm-explore-metrics-item-header__rate">
          <Tooltip title="calculates the average per-second speed of metric's change">
            <Switch
              label={<span>enable <code>rate()</code></span>}
              value={rateEnabled}
              onChange={onChangeRate}
            />
          </Tooltip>
        </div>
      )}
      <div className="vm-explore-metrics-item-header__close">
        <Tooltip title="close graph">
          <Button
            startIcon={<CloseIcon/>}
            variant="text"
            color="gray"
            size="small"
            onClick={handleClickRemove}
          />
        </Tooltip>
      </div>
    </div>
  );
};

export default ExploreMetricItemHeader;
