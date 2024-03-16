import React, { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";

interface Props {
  spanGaps: boolean,
  onChange: (value: boolean) => void,
}

const LinesConfigurator: FC<Props> = ({ spanGaps, onChange }) => {
  const { isMobile } = useDeviceDetect();

  return <div>
    <Switch
      value={spanGaps}
      onChange={onChange}
      label="Connect null values"
      fullWidth={isMobile}
    />
  </div>;
};

export default LinesConfigurator;
