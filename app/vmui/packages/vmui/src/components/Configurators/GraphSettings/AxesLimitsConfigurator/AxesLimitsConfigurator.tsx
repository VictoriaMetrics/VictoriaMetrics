import React, { FC, useCallback, useMemo } from "preact/compat";
import debounce from "lodash.debounce";
import { AxisRange, YaxisState } from "../../../../state/graph/reducer";
import "./style.scss";
import TextField from "../../../Main/TextField/TextField";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import classNames from "classnames";

interface AxesLimitsConfiguratorProps {
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void
}

const AxesLimitsConfigurator: FC<AxesLimitsConfiguratorProps> = ({ yaxis, setYaxisLimits, toggleEnableLimits }) => {
  const { isMobile } = useDeviceDetect();

  const axes = useMemo(() => Object.keys(yaxis.limits.range), [yaxis.limits.range]);

  const onChangeLimit = (value: string, axis: string, index: number) => {
    const newLimits = yaxis.limits.range;
    newLimits[axis][index] = +value;
    if (newLimits[axis][0] === newLimits[axis][1] || newLimits[axis][0] > newLimits[axis][1]) return;
    setYaxisLimits(newLimits);
  };
  const debouncedOnChangeLimit = useCallback(debounce(onChangeLimit, 500), [yaxis.limits.range]);

  const createHandlerOnchangeAxis = (axis: string, index: number) => (val: string) => {
    debouncedOnChangeLimit(val, axis, index);
  };

  return <div
    className={classNames({
      "vm-axes-limits": true,
      "vm-axes-limits_mobile": isMobile
    })}
  >
    <Switch
      value={yaxis.limits.enable}
      onChange={toggleEnableLimits}
      label="Fix the limits for y-axis"
      fullWidth={isMobile}
    />
    <div className="vm-axes-limits-list">
      {axes.map(axis => (
        <div
          className="vm-axes-limits-list__inputs"
          key={axis}
        >
          <TextField
            label={`Min ${axis}`}
            type="number"
            disabled={!yaxis.limits.enable}
            value={yaxis.limits.range[axis][0]}
            onChange={createHandlerOnchangeAxis(axis, 0)}
          />
          <TextField
            label={`Max ${axis}`}
            type="number"
            disabled={!yaxis.limits.enable}
            value={yaxis.limits.range[axis][1]}
            onChange={createHandlerOnchangeAxis(axis, 1)}
          />
        </div>
      ))}
    </div>
  </div>;
};

export default AxesLimitsConfigurator;
