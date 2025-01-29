import React, { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";

interface Props {
  spanGaps: boolean,
  onChange: (value: boolean) => void,
}

const LinesConfigurator: FC<Props> = ({ spanGaps, onChange }) => {
  const { isMobile } = useDeviceDetect();

  return (
    <div className="vm-graph-settings-row">
      <span className="vm-graph-settings-row__label">Connect null values</span>
      <Switch
        value={spanGaps}
        onChange={onChange}
        label={spanGaps ? "Enabled" : "Disabled"}
        fullWidth={isMobile}
      />
    </div>
  );
};

export default LinesConfigurator;
