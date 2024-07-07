import React, { forwardRef, useCallback, useImperativeHandle, useState } from "preact/compat";
import { DisplayType, ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { InfoIcon, RestartIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import { DEFAULT_MAX_SERIES } from "../../../../constants/graph";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { ChildComponentHandle } from "../GlobalSettings";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../../state/customPanel/CustomPanelStateContext";

interface ServerConfiguratorProps {
  onClose: () => void
}

const fields: {label: string, type: DisplayType}[] = [
  { label: "Graph", type: DisplayType.chart },
  { label: "JSON", type: DisplayType.code },
  { label: "Table", type: DisplayType.table }
];

const LimitsConfigurator = forwardRef<ChildComponentHandle, ServerConfiguratorProps>(({ onClose }, ref) => {
  const { isMobile } = useDeviceDetect();

  const { seriesLimits } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const [limits, setLimits] = useState(seriesLimits);
  const [error, setError] = useState({
    table: "",
    chart: "",
    code: ""
  });

  const handleReset = () => {
    setLimits(DEFAULT_MAX_SERIES);
  };

  const createChangeHandler = (type: DisplayType) =>  (val: string) => {
    const value = val || "";
    setError(prev => ({ ...prev, [type]: +value < 0 ? ErrorTypes.positiveNumber : "" }));
    setLimits({
      ...limits,
      [type]: !value ? Infinity : value
    });
  };

  const handleApply = useCallback(() => {
    customPanelDispatch({ type: "SET_SERIES_LIMITS", payload: limits });
    onClose();
  }, [limits]);

  useImperativeHandle(ref, () => ({ handleApply }), [handleApply]);

  return (
    <div className="vm-limits-configurator">
      <div className="vm-server-configurator__title">
        Series limits by tabs
        <Tooltip title="Set to 0 to disable the limit">
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<InfoIcon/>}
          />
        </Tooltip>
        <div className="vm-limits-configurator-title__reset">
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<RestartIcon/>}
            onClick={handleReset}
          >
            Reset limits
          </Button>
        </div>
      </div>
      <div
        className={classNames({
          "vm-limits-configurator__inputs": true,
          "vm-limits-configurator__inputs_mobile": isMobile
        })}
      >
        {fields.map(f => (
          <div key={f.type}>
            <TextField
              label={f.label}
              value={limits[f.type]}
              error={error[f.type]}
              onChange={createChangeHandler(f.type)}
              onEnter={handleApply}
              type="number"
            />
          </div>
        ))}
      </div>
    </div>
  );
});

export default LimitsConfigurator;
