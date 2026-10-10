import { createPortal, FC, useEffect } from "preact/compat";
import classNames from "classnames";
import TenantsConfiguration
  from "../../components/Configurators/GlobalSettings/TenantsConfiguration/TenantsConfiguration";
import StepConfigurator from "../../components/Configurators/StepConfigurator/StepConfigurator";
import { TimeSelector } from "../../components/Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import CardinalityDatePicker from "../../components/Configurators/CardinalityDatePicker/CardinalityDatePicker";
import GlobalSettings, { GlobalSettingsHandle } from "../../components/Configurators/GlobalSettings/GlobalSettings";
import ShortcutKeys from "../../components/Main/ShortcutKeys/ShortcutKeys";
import { ControlsProps } from "../Header/HeaderControls/HeaderControls";
import { useRef, useState } from "react";
import TimeZonePreview from "../../components/Configurators/GlobalSettings/TimeZonePreview/TimeZonePreview";
import ServerUrlDeprecationWarning
  from "../../components/Configurators/GlobalSettings/ServerConfigurator/ServerUrlDeprecationWarning";

const ControlsMainLayout: FC<ControlsProps> = ({
  displaySidebar,
  isMobile,
  headerSetup,
  accountIds,
}) => {
  const settingsRef = useRef<GlobalSettingsHandle>(null);

  const [warningContainer, setWarningContainer] = useState<HTMLDivElement | null>(null);

  useEffect(() => {
    const header = document.querySelector(".vm-header");
    if (!header) return;

    const container = document.createElement("div");
    header.after(container);
    setWarningContainer(container);

    return () => container.remove();
  }, []);

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
      <GlobalSettings ref={settingsRef}/>
      {!displaySidebar && <ShortcutKeys/>}

      {/* TODO: Remove this warning when Server URL editing is removed.
          See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11735 */}
      {warningContainer && createPortal(
        <ServerUrlDeprecationWarning
          onOpenSettings={() => settingsRef.current?.open()}
        />,
        warningContainer
      )}
    </div>
  );
};

export default ControlsMainLayout;
