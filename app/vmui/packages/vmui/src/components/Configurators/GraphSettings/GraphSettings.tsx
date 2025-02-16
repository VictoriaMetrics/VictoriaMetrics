import React, { FC, useRef } from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator/AxesLimitsConfigurator";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";
import { SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import "./style.scss";
import Tooltip from "../../Main/Tooltip/Tooltip";
import useBoolean from "../../../hooks/useBoolean";
import LinesConfigurator from "./LinesConfigurator/LinesConfigurator";
import GraphTypeSwitcher from "./GraphTypeSwitcher/GraphTypeSwitcher";
import { MetricResult } from "../../../api/types";
import { isHistogramData } from "../../../utils/metric";
import LegendConfigs from "../../Chart/Line/Legend/LegendConfigs/LegendConfigs";
import Modal from "../../Main/Modal/Modal";

const title = "Graph settings";

interface GraphSettingsProps {
  data: MetricResult[],
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void,
  spanGaps: {
    value: boolean,
    onChange: (value: boolean) => void,
  },
  isHistogram?: boolean,
}

const GraphSettings: FC<GraphSettingsProps> = ({ data, yaxis, setYaxisLimits, toggleEnableLimits, spanGaps }) => {
  const popperRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLDivElement>(null);
  const displayHistogramMode = isHistogramData(data);

  const {
    value: isOpenSettings,
    setTrue: handleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  return (
    <div className="vm-graph-settings">
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={handleOpen}
            ariaLabel="settings"
          />
        </div>
      </Tooltip>
      {isOpenSettings && (
        <Modal
          onClose={handleClose}
          title={title}
          className="vm-graph-settings-modal"
        >
          <div
            className="vm-graph-settings-body"
            ref={popperRef}
          >
            <div className="vm-graph-settings-body-section">
              <h3 className="vm-graph-settings-body-section__title">Chart Options</h3>
              <div className="vm-graph-settings-body-section-content">
                <AxesLimitsConfigurator
                  yaxis={yaxis}
                  setYaxisLimits={setYaxisLimits}
                  toggleEnableLimits={toggleEnableLimits}
                />
                <LinesConfigurator
                  spanGaps={spanGaps.value}
                  onChange={spanGaps.onChange}
                />
                {displayHistogramMode && <GraphTypeSwitcher onChange={handleClose}/>}
              </div>
            </div>

            <div className="vm-graph-settings-body-section">
              <h3 className="vm-graph-settings-body-section__title">Legend Options</h3>
              <div className="vm-graph-settings-body-section-content">
                <LegendConfigs data={data}/>
              </div>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default GraphSettings;
