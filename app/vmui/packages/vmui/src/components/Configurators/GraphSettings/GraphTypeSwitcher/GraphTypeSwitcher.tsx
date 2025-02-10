import React, { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { useSearchParams } from "react-router-dom";
import { useChangeDisplayMode } from "./useChangeDisplayMode";

type Props = {
  onChange: () => void;
}

const GraphTypeSwitcher: FC<Props> = ({ onChange }) => {
  const { isMobile } = useDeviceDetect();

  const { handleChange } = useChangeDisplayMode();
  const [searchParams] = useSearchParams();

  const value = !searchParams.get("display_mode");

  const handleChangeMode = (val: boolean) => {
    handleChange(val, onChange);
  };

  return (
    <div className="vm-graph-settings-row">
      <span className="vm-graph-settings-row__label">Histogram mode</span>
      <Switch
        value={value}
        onChange={handleChangeMode}
        label={value ? "Enabled" : "Disabled"}
        fullWidth={isMobile}
      />
    </div>
  );
};

export default GraphTypeSwitcher;
