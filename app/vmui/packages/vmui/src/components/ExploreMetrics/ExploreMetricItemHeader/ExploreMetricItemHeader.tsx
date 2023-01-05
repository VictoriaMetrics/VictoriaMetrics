import React, { FC, useRef, useState } from "preact/compat";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";
import { ArrowDownIcon, CloseIcon, ResizeIcon } from "../../Main/Icons";
import Popper from "../../Main/Popper/Popper";
import ExploreMetricLayouts from "../ExploreMetricLayouts/ExploreMetricLayouts";

interface ExploreMetricItemControlsProps {
  name: string
  index: number
  isBucket: boolean
  rateEnabled: boolean
  showLegend: boolean
  size: string
  onChangeRate: (val: boolean) => void
  onChangeLegend: (val: boolean) => void
  onRemoveItem: (name: string) => void
  onChangeOrder: (name: string, oldIndex: number, newIndex: number) => void
  onChangeSize: (id: string) => void
}

const ExploreMetricItemHeader: FC<ExploreMetricItemControlsProps> = ({
  name,
  index,
  isBucket,
  rateEnabled,
  showLegend,
  size,
  onChangeRate,
  onChangeLegend,
  onRemoveItem,
  onChangeOrder,
  onChangeSize
}) => {

  const layoutButtonRef = useRef<HTMLDivElement>(null);
  const [openPopper, setOpenPopper] = useState(false);
  const handleClickRemove = () => {
    onRemoveItem(name);
  };

  const handleOrderDown = () => {
    onChangeOrder(name, index, index + 1);
  };

  const handleOrderUp = () => {
    onChangeOrder(name, index, index - 1);
  };

  const handleTogglePopper = () => {
    setOpenPopper(prev => !prev);
  };

  const handleClosePopper = () => {
    setOpenPopper(false);
  };

  const handleChangeSize = (id: string) => {
    onChangeSize(id);
    handleClosePopper();
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
        <Tooltip title="calculates the average per-second speed of metric's change">
          <Switch
            label={<span>enable <code>rate()</code></span>}
            value={rateEnabled}
            onChange={onChangeRate}
          />
        </Tooltip>
      )}
      <Switch
        label="show legend"
        value={showLegend}
        onChange={onChangeLegend}
      />
      <div className="vm-explore-metrics-item-header__layout">
        <Tooltip title="change size the graph">
          <div ref={layoutButtonRef}>
            <Button
              startIcon={<ResizeIcon/>}
              variant="text"
              color="gray"
              size="small"
              onClick={handleTogglePopper}
            />
          </div>
        </Tooltip>
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

      <Popper
        open={openPopper}
        onClose={handleClosePopper}
        placement="bottom-right"
        buttonRef={layoutButtonRef}
      >
        <ExploreMetricLayouts
          value={size}
          onChange={handleChangeSize}
        />
      </Popper>
    </div>
  );
};

export default ExploreMetricItemHeader;
