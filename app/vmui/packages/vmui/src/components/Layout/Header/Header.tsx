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
import { LogoIcon } from "../../Main/Icons";
import { getCssVariable } from "../../../utils/theme";
import Tabs from "../../Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";

const Header: FC = () => {
  const primaryColor = getCssVariable("color-primary");
  const appModeEnable = getAppModeEnable();

  const { headerStyles: {
    background = appModeEnable ? "#FFF" : primaryColor,
    color = appModeEnable ? primaryColor : "#FFF",
  } = {} } = getAppModeParams();

  const navigate = useNavigate();
  const { search, pathname } = useLocation();
  const routes = useMemo(() => ([
    {
      label: "Custom panel",
      value: router.home,
    },
    {
      label: "Dashboards",
      value: router.dashboards,
      hide: appModeEnable
    },
    {
      label: "Cardinality",
      value: router.cardinality,
    },
    {
      label: "Top queries",
      value: router.topQueries,
    },
    {
      label: "Trace analyzer",
      value: router.trace,
    }
  ]), [appModeEnable]);

  const [activeMenu, setActiveMenu] = useState(pathname);

  const handleChangeTab = (value: string) => {
    setActiveMenu(value);
    navigate(value);
  };

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
        style={{ color }}
      >
        <div
          className="vm-header-logo__icon"
          onClick={onClickLogo}
        >
          <LogoIcon/>
        </div>
        <a
          className="vm-header-logo__issue"
          target="_blank"
          href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new"
          rel="noreferrer"
        >
            create an issue
        </a>
      </div>
    )}
    <div className="vm-header-nav">
      <Tabs
        activeItem={activeMenu}
        items={routes.filter(r => !r.hide)}
        color={color}
        onChange={handleChangeTab}
      />
    </div>
    <div className="vm-header__settings">
      {headerSetup?.timeSelector && <TimeSelector/>}
      {headerSetup?.cardinalityDatePicker && <CardinalityDatePicker/>}
      {headerSetup?.executionControls && <ExecutionControls/>}
      <GlobalSettings/>
      <ShortcutKeys/>
    </div>
  </header>;
};

export default Header;
