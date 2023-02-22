import React, { FC, useMemo } from "preact/compat";
import { ExecutionControls } from "../../Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import { TimeSelector } from "../../Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import GlobalSettings from "../../Configurators/GlobalSettings/GlobalSettings";
import { useLocation, useNavigate } from "react-router-dom";
import router, { RouterOptions, routerOptions } from "../../../router";
import ShortcutKeys from "../../Main/ShortcutKeys/ShortcutKeys";
import { getAppModeEnable, getAppModeParams } from "../../../utils/app-mode";
import CardinalityDatePicker from "../../Configurators/CardinalityDatePicker/CardinalityDatePicker";
import { LogoFullIcon } from "../../Main/Icons";
import { getCssVariable } from "../../../utils/theme";
import "./style.scss";
import classNames from "classnames";
import StepConfigurator from "../../Configurators/StepConfigurator/StepConfigurator";
import { useAppState } from "../../../state/common/StateContext";
import HeaderNav from "./HeaderNav/HeaderNav";
import TenantsConfiguration from "../../Configurators/GlobalSettings/TenantsConfiguration/TenantsConfiguration";
import { useFetchAccountIds } from "../../Configurators/GlobalSettings/TenantsConfiguration/hooks/useFetchAccountIds";
import useResize from "../../../hooks/useResize";
import SidebarHeader from "./SidebarNav/SidebarHeader";

const Header: FC = () => {
  const windowSize = useResize(document.body);
  const displaySidebar = useMemo(() => window.innerWidth < 1000, [windowSize]);

  const { isDarkTheme } = useAppState();
  const appModeEnable = getAppModeEnable();
  const { accountIds } = useFetchAccountIds();

  const primaryColor = useMemo(() => {
    const variable = isDarkTheme ? "color-background-block" : "color-primary";
    return getCssVariable(variable);
  }, [isDarkTheme]);

  const { background, color } = useMemo(() => {
    const { headerStyles: {
      background = appModeEnable ? "#FFF" : primaryColor,
      color = appModeEnable ? primaryColor : "#FFF",
    } = {} } = getAppModeParams();

    return { background, color };
  }, [primaryColor]);

  const navigate = useNavigate();
  const { pathname } = useLocation();

  const headerSetup = useMemo(() => {
    return ((routerOptions[pathname] || {}) as RouterOptions).header || {};
  }, [pathname]);

  const onClickLogo = () => {
    navigate({ pathname: router.home });
    window.location.reload();
  };

  return <header
    className={classNames({
      "vm-header": true,
      "vm-header_app": appModeEnable,
      "vm-header_dark": isDarkTheme
    })}
    style={{ background, color }}
  >
    {displaySidebar ? (
      <SidebarHeader
        background={background}
        color={color}
        onClickLogo={onClickLogo}
      />
    ) : (
      <>
        {!appModeEnable && (
          <div
            className="vm-header-logo"
            onClick={onClickLogo}
            style={{ color }}
          >
            <LogoFullIcon/>
          </div>
        )}
        <HeaderNav
          color={color}
          background={background}
        />
      </>
    )}
    <div className="vm-header__settings">
      {headerSetup?.tenant && <TenantsConfiguration accountIds={accountIds}/>}
      {headerSetup?.stepControl && <StepConfigurator/>}
      {headerSetup?.timeSelector && <TimeSelector/>}
      {headerSetup?.cardinalityDatePicker && <CardinalityDatePicker/>}
      {headerSetup?.executionControls && <ExecutionControls/>}
      {!displaySidebar && <GlobalSettings/>}
      {!displaySidebar && <ShortcutKeys/>}
    </div>
  </header>;
};

export default Header;
