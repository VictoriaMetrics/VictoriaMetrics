import React, { FC, useRef, useState } from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator/AxesLimitsConfigurator";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { CloseIcon, SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import useClickOutside from "../../../hooks/useClickOutside";
import Popper from "../../Main/Popper/Popper";
import "./style.scss";

const title = "Axes Index";

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

  return (
    <div className="vm-graph-settings">
      {/*<Tooltip title={title}>*/}
      <div ref={buttonRef}>
        <Button onClick={() => setOpenPopper(true)}>
          <SettingsIcon/>
        </Button>
      </div>
      {/*</Tooltip>*/}
      <Popper
        open={openPopper}
        buttonRef={buttonRef}
        placement="left-start"
        onClose={() => setOpenPopper(false)}
      >
        <div
          className="vm-graph-settings-popper"
          ref={popperRef}
        >
          <div className="vm-graph-settings-popper-header">
            <div className="vm-graph-settings-popper-header__title">
              {title}
            </div>
            <Button onClick={() => setOpenPopper(false)}>
              <CloseIcon/>
            </Button>
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
