import { FC, useMemo } from "preact/compat";
import { RouterOptions, routerOptions, RouterOptionsHeader } from "../../../router";
import { useLocation } from "react-router-dom";
import {
  useFetchAccountIds
} from "../../../components/Configurators/GlobalSettings/TenantsConfiguration/hooks/useFetchAccountIds";
import Button from "../../../components/Main/Button/Button";
import { MoreIcon } from "../../../components/Main/Icons";
import classNames from "classnames";
import { getAppModeEnable } from "../../../utils/app-mode";
import Modal from "../../../components/Main/Modal/Modal";
import useBoolean from "../../../hooks/useBoolean";
import { HeaderProps } from "../Header";
import "./style.scss";

export interface ControlsProps {
  displaySidebar: boolean;
  isMobile?: boolean;
  headerSetup?: RouterOptionsHeader;
  accountIds?: string[];
}

const HeaderControls: FC<ControlsProps & HeaderProps> = ({
  controlsComponent: ControlsComponent,
  isMobile,
  ...props
}) => {
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

  const controls = (
    <ControlsComponent
      {...props}
      isMobile={isMobile}
      accountIds={accountIds}
      headerSetup={headerSetup}
    />
  );

  if (isMobile) {
    return (
      <>
        <div>
          <Button
            className={classNames({
              "vm-header-button": !appModeEnable
            })}
            startIcon={<MoreIcon/>}
            onClick={handleToggleList}
            ariaLabel={"controls"}
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
          {controls}
        </Modal>
      </>
    );
  }

  return controls;
};

export default HeaderControls;
