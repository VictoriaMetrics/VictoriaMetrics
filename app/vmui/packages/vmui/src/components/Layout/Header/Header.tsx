import React, { FC, useMemo, useState } from "preact/compat";
import { ExecutionControls } from "../../Configurators/TimeRangeSettings/ExecutionControls/ExecutionControls";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";
import { TimeSelector } from "../../Configurators/TimeRangeSettings/TimeSelector/TimeSelector";
import GlobalSettings from "../../Configurators/GlobalSettings/GlobalSettings";
import { useLocation, useNavigate } from "react-router-dom";
import router, { RouterOptions, routerOptions } from "../../../router";
import { useEffect } from "react";
import ShortcutKeys from "../../Main/ShortcutKeys/ShortcutKeys";
import { getAppModeEnable, getAppModeParams } from "../../../utils/app-mode";
import CardinalityDatePicker from "../../Configurators/CardinalityDatePicker/CardinalityDatePicker";
import { LogoFullIcon } from "../../Main/Icons";
import { getCssVariable } from "../../../utils/theme";
import Tabs from "../../Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import { useDashboardsState } from "../../../state/dashboards/DashboardsStateContext";
import StepConfigurator from "../../Configurators/StepConfigurator/StepConfigurator";

const Header: FC = () => {
  const primaryColor = getCssVariable("color-primary");
  const appModeEnable = getAppModeEnable();
  const { dashboardsSettings } = useDashboardsState();

  const { headerStyles: {
    background = appModeEnable ? "#FFF" : primaryColor,
    color = appModeEnable ? primaryColor : "#FFF",
  } = {} } = getAppModeParams();

  const navigate = useNavigate();
  const { search, pathname } = useLocation();
  const routes = useMemo(() => ([
    {
      label: routerOptions[router.home].title,
      value: router.home,
    },
    {
      label: routerOptions[router.metrics].title,
      value: router.metrics,
    },
    {
      label: routerOptions[router.cardinality].title,
      value: router.cardinality,
    },
    {
      label: routerOptions[router.topQueries].title,
      value: router.topQueries,
    },
    {
      label: routerOptions[router.trace].title,
      value: router.trace,
    },
    {
      label: routerOptions[router.dashboards].title,
      value: router.dashboards,
      hide: appModeEnable || !dashboardsSettings.length
    }
  ]), [appModeEnable, dashboardsSettings]);

  const [activeMenu, setActiveMenu] = useState(pathname);

  const headerSetup = useMemo(() => {
    return ((routerOptions[pathname] || {}) as RouterOptions).header || {};
  }, [pathname]);

  const onClickLogo = () => {
    navigateHandler(router.home);
    setQueryStringWithoutPageReload({});
    window.location.reload();
  };

  const navigateHandler = (pathname: string) => {
    navigate({ pathname, search: search });
  };

  useEffect(() => {
    setActiveMenu(pathname);
  }, [pathname]);

  return <header
    className={classNames({
      "vm-header": true,
      "vm-header_app": appModeEnable
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
    <div className="vm-header-nav">
      <Tabs
        isNavLink
        activeItem={activeMenu}
        items={routes.filter(r => !r.hide)}
        color={color}
      />
    </div>
    <div className="vm-header__settings">
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
