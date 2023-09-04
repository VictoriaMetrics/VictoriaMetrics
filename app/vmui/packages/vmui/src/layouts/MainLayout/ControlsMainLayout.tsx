import React, { FC } from "preact/compat";
import classNames from "classnames";
import TenantsConfiguration
  from "../../components/Configurators/GlobalSettings/TenantsConfiguration/TenantsConfiguration";
import StepConfigurator from "../../components/Configurators/StepConfigurator/StepConfigurator";
import { TimeSelector } from "../../components/Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import CardinalityDatePicker from "../../components/Configurators/CardinalityDatePicker/CardinalityDatePicker";
import { ExecutionControls } from "../../components/Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import GlobalSettings from "../../components/Configurators/GlobalSettings/GlobalSettings";
import ShortcutKeys from "../../components/Main/ShortcutKeys/ShortcutKeys";
import { ControlsProps } from "../Header/HeaderControls/HeaderControls";

const ControlsMainLayout: FC<ControlsProps> = ({
  displaySidebar,
  isMobile,
  headerSetup,
  accountIds
}) => {

  return (
    <div
      className={classNames({
        "vm-header-controls": true,
        "vm-header-controls_mobile": isMobile,
      })}
    >
      {headerSetup?.tenant && <TenantsConfiguration accountIds={accountIds || []}/>}
      {headerSetup?.stepControl && <StepConfigurator/>}
      {headerSetup?.timeSelector && <TimeSelector/>}
      {headerSetup?.cardinalityDatePicker && <CardinalityDatePicker/>}
      {headerSetup?.executionControls && <ExecutionControls/>}
      <GlobalSettings/>
      {!displaySidebar && <ShortcutKeys/>}
    </div>
  );
};

export default ControlsMainLayout;
