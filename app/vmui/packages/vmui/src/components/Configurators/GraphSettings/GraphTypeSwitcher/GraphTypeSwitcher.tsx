import React, { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { useSearchParams } from "react-router-dom";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";

type Props = {
  onChange: () => void;
}

const GraphTypeSwitcher: FC<Props> = ({ onChange }) => {
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();
  const dispatch = useTimeDispatch();

  const value = !searchParams.get("display_mode");

  const handleChangeMode = (val: boolean) => {
    val ? searchParams.delete("display_mode") : searchParams.set("display_mode", "lines");
    setSearchParams(searchParams);
    dispatch({ type: "RUN_QUERY" });
    onChange();
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
