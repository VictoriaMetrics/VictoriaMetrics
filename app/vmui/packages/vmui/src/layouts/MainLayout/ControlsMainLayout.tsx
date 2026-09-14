import { FC } from "preact/compat";
import classNames from "classnames";
import TenantsConfiguration
  from "../../components/Configurators/GlobalSettings/TenantsConfiguration/TenantsConfiguration";
import StepConfigurator from "../../components/Configurators/StepConfigurator/StepConfigurator";
import { TimeSelector } from "../../components/Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import CardinalityDatePicker from "../../components/Configurators/CardinalityDatePicker/CardinalityDatePicker";
import { ExecutionControls } from "../../components/Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import GlobalSettings, { GlobalSettingsHandle } from "../../components/Configurators/GlobalSettings/GlobalSettings";
import ShortcutKeys from "../../components/Main/ShortcutKeys/ShortcutKeys";
import { ControlsProps } from "../Header/HeaderControls/HeaderControls";
import { useRef } from "react";
import TimeZonePreview from "../../components/Configurators/GlobalSettings/TimeZonePreview/TimeZonePreview";

const ControlsMainLayout: FC<ControlsProps> = ({
  displaySidebar,
  isMobile,
  headerSetup,
  accountIds,
  closeModal,
}) => {
  const settingsRef = useRef<GlobalSettingsHandle>(null);

  return (
    <div
      className={classNames({
        "vm-header-controls": true,
        "vm-header-controls_mobile": isMobile,
      })}
    >
      {headerSetup?.tenant && <TenantsConfiguration accountIds={accountIds || []}/>}
      {headerSetup?.stepControl && <StepConfigurator/>}
      {headerSetup?.timeSelector && <TimeSelector onOpenSettings={() => settingsRef.current?.open()}/>}
      {headerSetup?.cardinalityDatePicker && <CardinalityDatePicker/>}
      <TimeZonePreview onOpenSettings={() => settingsRef.current?.open()}/>
      {headerSetup?.executionControls && <ExecutionControls
        tooltip={headerSetup?.executionControls?.tooltip}
        useAutorefresh={headerSetup?.executionControls?.useAutorefresh}
        closeModal={closeModal}
      />}
      <GlobalSettings ref={settingsRef}/>
      {!displaySidebar && <ShortcutKeys/>}
    </div>
  );
};

export default ControlsMainLayout;
