import React, { FC, useRef, useState } from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator/AxesLimitsConfigurator";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { CloseIcon, SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import useClickOutside from "../../../hooks/useClickOutside";
import Popper from "../../Main/Popper/Popper";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";

const title = "Axes settings";

interface GraphSettingsProps {
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void
}

const GraphSettings: FC<GraphSettingsProps> = ({ yaxis, setYaxisLimits, toggleEnableLimits }) => {
  const popperRef = useRef<HTMLDivElement>(null);
  const [openPopper, setOpenPopper] = useState(false);
  const buttonRef = useRef<HTMLDivElement>(null);
  useClickOutside(popperRef, () => setOpenPopper(false), buttonRef);

  const handleOpen = () => {
    setOpenPopper(true);
  };

  const handleClose = () => {
    setOpenPopper(false);
  };

  return (
    <div className="vm-graph-settings">
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={handleOpen}
          />
        </div>
      </Tooltip>
      <Popper
        open={openPopper}
        buttonRef={buttonRef}
        placement="bottom-right"
        onClose={handleClose}
      >
        <div
          className="vm-graph-settings-popper"
          ref={popperRef}
        >
          <div className="vm-popper-header">
            <h3 className="vm-popper-header__title">
              {title}
            </h3>
            <Button
              size="small"
              startIcon={<CloseIcon/>}
              onClick={handleClose}
            />
          </div>
          <div className="vm-graph-settings-popper__body">
            <AxesLimitsConfigurator
              yaxis={yaxis}
              setYaxisLimits={setYaxisLimits}
              toggleEnableLimits={toggleEnableLimits}
            />
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default GraphSettings;
