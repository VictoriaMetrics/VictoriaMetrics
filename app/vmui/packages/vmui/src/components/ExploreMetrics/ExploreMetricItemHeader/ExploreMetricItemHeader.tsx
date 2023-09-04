import React, { FC } from "preact/compat";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";
import { ArrowDownIcon, CloseIcon, MinusIcon, MoreIcon, PlusIcon } from "../../Main/Icons";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Modal from "../../Main/Modal/Modal";
import useBoolean from "../../../hooks/useBoolean";

interface ExploreMetricItemControlsProps {
  name: string
  index: number
  length: number
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
  length,
  isBucket,
  rateEnabled,
  onChangeRate,
  onRemoveItem,
  onChangeOrder,
}) => {
  const { isMobile } = useDeviceDetect();

  const {
    value: openOptions,
    setTrue: handleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

  const handleClickRemove = () => {
    onRemoveItem(name);
  };

  const handleOrderDown = () => {
    onChangeOrder(name, index, index + 1);
  };

  const handleOrderUp = () => {
    onChangeOrder(name, index, index - 1);
  };

  if (isMobile) {
    return (
      <div className="vm-explore-metrics-item-header vm-explore-metrics-item-header_mobile">
        <div className="vm-explore-metrics-item-header__name">{name}</div>
        <Button
          variant="text"
          size="small"
          startIcon={<MoreIcon/>}
          onClick={handleOpenOptions}
          ariaLabel="open panel settings"
        />
        {openOptions && (
          <Modal
            title={name}
            onClose={handleCloseOptions}
          >
            <div className="vm-explore-metrics-item-header-modal">
              <div className="vm-explore-metrics-item-header-modal-order">
                <Button
                  startIcon={<MinusIcon/>}
                  variant="outlined"
                  onClick={handleOrderUp}
                  disabled={index === 0}
                  ariaLabel="move graph up"
                />
                <p>position:
                  <span className="vm-explore-metrics-item-header-modal-order__index">#{index + 1}</span>
                </p>
                <Button
                  endIcon={<PlusIcon/>}
                  variant="outlined"
                  onClick={handleOrderDown}
                  disabled={index === length - 1}
                  ariaLabel="move graph down"
                />
              </div>
              {!isBucket && (
                <div className="vm-explore-metrics-item-header-modal__rate">
                  <Switch
                    label={<span>enable <code>rate()</code></span>}
                    value={rateEnabled}
                    onChange={onChangeRate}
                    fullWidth
                  />
                  <p>
                    calculates the average per-second speed of metrics change
                  </p>
                </div>
              )}
              <Button
                startIcon={<CloseIcon/>}
                color="error"
                variant="outlined"
                onClick={handleClickRemove}
                fullWidth
              >
                  Remove graph
              </Button>
            </div>
          </Modal>
        )}
      </div>
    );
  }

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
            ariaLabel="move graph up"
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
            ariaLabel="move graph down"
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
            ariaLabel="close graph"
          />
        </Tooltip>
      </div>
    </div>
  );
};

export default ExploreMetricItemHeader;
