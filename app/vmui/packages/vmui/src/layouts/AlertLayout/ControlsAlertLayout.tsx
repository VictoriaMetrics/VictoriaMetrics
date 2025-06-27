import React, { FC } from "preact/compat";
import classNames from "classnames";
import GlobalSettings from "../../components/Configurators/GlobalSettings/GlobalSettings";
import { ControlsProps } from "../Header/HeaderControls/HeaderControls";
import { ExecutionControls } from "../../components/Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";

const ControlsAlertLayout: FC<ControlsProps> = ({ isMobile }) => {

  return (
    <div
      className={classNames({
        "vm-header-controls": true,
        "vm-header-controls_mobile": isMobile,
      })}
    >
      <ExecutionControls/>
      <GlobalSettings/>
    </div>
  );
};

export default ControlsAlertLayout;
