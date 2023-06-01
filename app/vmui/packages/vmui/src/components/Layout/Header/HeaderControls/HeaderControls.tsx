import React, { FC, useMemo } from "preact/compat";
import { RouterOptions, routerOptions, RouterOptionsHeader } from "../../../../router";
import TenantsConfiguration from "../../../Configurators/GlobalSettings/TenantsConfiguration/TenantsConfiguration";
import StepConfigurator from "../../../Configurators/StepConfigurator/StepConfigurator";
import { TimeSelector } from "../../../Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import CardinalityDatePicker from "../../../Configurators/CardinalityDatePicker/CardinalityDatePicker";
import { ExecutionControls } from "../../../Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import GlobalSettings from "../../../Configurators/GlobalSettings/GlobalSettings";
import ShortcutKeys from "../../../Main/ShortcutKeys/ShortcutKeys";
import { useLocation } from "react-router-dom";
import { useFetchAccountIds } from "../../../Configurators/GlobalSettings/TenantsConfiguration/hooks/useFetchAccountIds";
import Button from "../../../Main/Button/Button";
import { MoreIcon } from "../../../Main/Icons";
import "./style.scss";
import classNames from "classnames";
import { getAppModeEnable } from "../../../../utils/app-mode";
import Modal from "../../../Main/Modal/Modal";
import useBoolean from "../../../../hooks/useBoolean";

interface HeaderControlsProp {
  displaySidebar: boolean
  isMobile?: boolean
  headerSetup?: RouterOptionsHeader
  accountIds?: string[]
}

const Controls: FC<HeaderControlsProp> = ({
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

const HeaderControls: FC<HeaderControlsProp> = (props) => {
  const appModeEnable = getAppModeEnable();
  const { pathname } = useLocation();
  const { accountIds } = useFetchAccountIds();

  const {
    value: openList,
    toggle: handleToggleList,
    setFalse: handleCloseList,
  } = useBoolean(false);

  const headerSetup = useMemo(() => {
    return ((routerOptions[pathname] || {}) as RouterOptions).header || {};
  }, [pathname]);

  if (props.isMobile) {
    return (
      <>
        <div>
          <Button
            className={classNames({
              "vm-header-button": !appModeEnable
            })}
            startIcon={<MoreIcon/>}
            onClick={handleToggleList}
          />
        </div>
        <Modal
          title={"Controls"}
          onClose={handleCloseList}
          isOpen={openList}
          className={classNames({
            "vm-header-controls-modal": true,
            "vm-header-controls-modal_open": openList,
          })}
        >
          <Controls
            {...props}
            accountIds={accountIds}
            headerSetup={headerSetup}
          />
        </Modal>
      </>
    );
  }

  return <Controls
    {...props}
    accountIds={accountIds}
    headerSetup={headerSetup}
  />;
};

export default HeaderControls;
