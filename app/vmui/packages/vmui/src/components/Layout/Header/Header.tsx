import React, { FC, useMemo } from "preact/compat";
import { ExecutionControls } from "../../Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";
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

const Header: FC = () => {
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
  const { search, pathname } = useLocation();

  const headerSetup = useMemo(() => {
    return ((routerOptions[pathname] || {}) as RouterOptions).header || {};
  }, [pathname]);

  const onClickLogo = () => {
    navigate({ pathname: router.home, search: search });
    setQueryStringWithoutPageReload({});
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
    <div className="vm-header__settings">
      {headerSetup?.tenant && <TenantsConfiguration accountIds={accountIds}/>}
      {headerSetup?.stepControl && <StepConfigurator/>}
      {headerSetup?.timeSelector && <TimeSelector/>}
      {headerSetup?.cardinalityDatePicker && <CardinalityDatePicker/>}
      {headerSetup?.executionControls && <ExecutionControls/>}
      <GlobalSettings/>
      <ShortcutKeys/>
    </div>
  </header>;
};

export default Header;
