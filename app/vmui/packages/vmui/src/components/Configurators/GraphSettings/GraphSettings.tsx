import React, { FC, useRef } from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator/AxesLimitsConfigurator";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Popper from "../../Main/Popper/Popper";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";
import useBoolean from "../../../hooks/useBoolean";
import LinesConfigurator from "./LinesConfigurator/LinesConfigurator";

const title = "Graph settings";

interface GraphSettingsProps {
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void,
  spanGaps: {
    value: boolean,
    onChange: (value: boolean) => void,
  },
}

const GraphSettings: FC<GraphSettingsProps> = ({ yaxis, setYaxisLimits, toggleEnableLimits, spanGaps }) => {
  const popperRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openPopper,
    toggle: toggleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  return (
    <div className="vm-graph-settings">
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpen}
            ariaLabel="settings"
          />
        </div>
      </Tooltip>
      <Popper
        open={openPopper}
        buttonRef={buttonRef}
        placement="bottom-right"
        onClose={handleClose}
        title={title}
      >
        <div
          className="vm-graph-settings-popper"
          ref={popperRef}
        >
          <div className="vm-graph-settings-popper__body">
            <AxesLimitsConfigurator
              yaxis={yaxis}
              setYaxisLimits={setYaxisLimits}
              toggleEnableLimits={toggleEnableLimits}
            />
            <LinesConfigurator
              spanGaps={spanGaps.value}
              onChange={spanGaps.onChange}
            />
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default GraphSettings;
