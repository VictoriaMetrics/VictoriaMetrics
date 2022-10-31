import React, { FC, useMemo, useState } from "preact/compat";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Link from "@mui/material/Link";
import Toolbar from "@mui/material/Toolbar";
import { ExecutionControls } from "../Configurators/TimeRangeSettings/ExecutionControls";
import Logo from "../Main/Icons/Logo";
import { setQueryStringWithoutPageReload } from "../../utils/query-string";
import { TimeSelector } from "../Configurators/TimeRangeSettings/TimeSelector";
import GlobalSettings from "../Configurators/GlobalSettings/GlobalSettings";
import { Link as RouterLink, useLocation, useNavigate } from "react-router-dom";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import router, { RouterOptions, routerOptions } from "../../router";
import DatePicker from "../Main/DatePicker/DatePicker";
import { useCardinalityState, useCardinalityDispatch } from "../../state/cardinality/CardinalityStateContext";
import { useEffect } from "react";
import ShortcutKeys from "../Main/ShortcutKeys/ShortcutKeys";
import { getAppModeEnable, getAppModeParams } from "../../utils/app-mode";

const classes = {
  logo: {
    position: "relative",
    display: "flex",
    alignItems: "center",
    color: "#fff",
    cursor: "pointer",
    width: "100%",
    marginBottom: "2px"
  },
  issueLink: {
    textAlign: "center",
    fontSize: "10px",
    opacity: ".4",
    color: "inherit",
    textDecoration: "underline",
    transition: ".2s opacity",
    whiteSpace: "nowrap",
    "&:hover": {
      opacity: ".8",
    }
  },
  menuLink: {
    display: "block",
    padding: "16px 8px",
    color: "white",
    fontSize: "11px",
    textDecoration: "none",
    cursor: "pointer",
    textTransform: "uppercase",
    borderRadius: "4px",
    transition: ".2s background",
    "&:hover": {
      boxShadow: "rgba(0, 0, 0, 0.15) 0px 2px 8px"
    }
  }
};

const Header: FC = () => {

  const appModeEnable = getAppModeEnable();
  const { headerStyles: {
    background = appModeEnable ? "#FFF" : "primary.main",
    color = appModeEnable ? "primary.main" : "#FFF",
  } = {} } = getAppModeParams();

  const { date } = useCardinalityState();
  const cardinalityDispatch = useCardinalityDispatch();

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
    }
  ]), [appModeEnable]);

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

  return <AppBar
    position="static"
    sx={{ px: 1, boxShadow: "none", bgcolor: background, color }}
  >
    <Toolbar>
      {!appModeEnable && (
        <Box
          display="grid"
          alignItems="center"
          justifyContent="center"
        >
          <Box
            onClick={onClickLogo}
            sx={classes.logo}
          >
            <Logo style={{ color: "inherit", width: "100%" }}/>
          </Box>
          <Link
            sx={classes.issueLink}
            target="_blank"
            href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new"
          >
            create an issue
          </Link>
        </Box>
      )}
      <Box
        ml={appModeEnable ? 0 : 8}
        flexGrow={1}
      >
        <Tabs
          value={activeMenu}
          textColor="inherit"
          TabIndicatorProps={{ style: { background: color } }}
          onChange={(e, val) => setActiveMenu(val)}
        >
          {routes.filter(r => !r.hide).map(r => (
            <Tab
              key={`${r.label}_${r.value}`}
              label={r.label}
              value={r.value}
              component={RouterLink}
              to={`${r.value}${search}`}
              sx={{ color }}
            />
          ))}
        </Tabs>
      </Box>
      <Box
        display="flex"
        gap={1}
        alignItems="center"
        mr={0}
        ml={4}
      >
        {headerSetup?.timeSelector && <TimeSelector/>}
        {headerSetup?.datePicker && (
          <DatePicker
            date={date}
            onChange={(val) => cardinalityDispatch({ type: "SET_DATE", payload: val })}
          />
        )}
        {headerSetup?.executionControls && <ExecutionControls/>}
        {headerSetup?.globalSettings && !appModeEnable && <GlobalSettings/>}
        <ShortcutKeys/>
      </Box>
    </Toolbar>
  </AppBar>;
};

export default Header;
