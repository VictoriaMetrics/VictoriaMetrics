import React, {FC, useMemo, useState} from "preact/compat";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Link from "@mui/material/Link";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import {ExecutionControls} from "../CustomPanel/Configurator/Time/ExecutionControls";
import Logo from "../common/Logo";
import {setQueryStringWithoutPageReload} from "../../utils/query-string";
import {TimeSelector} from "../CustomPanel/Configurator/Time/TimeSelector";
import GlobalSettings from "../CustomPanel/Configurator/Settings/GlobalSettings";
import {Link as RouterLink, useLocation, useNavigate} from "react-router-dom";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import router, {RouterOptions, routerOptions} from "../../router/index";
import DatePicker from "../Main/DatePicker/DatePicker";
import {useCardinalityState, useCardinalityDispatch} from "../../state/cardinality/CardinalityStateContext";
import {useEffect} from "react";

const classes = {
  logo: {
    position: "relative",
    display: "flex",
    alignItems: "center",
    color: "#fff",
    cursor: "pointer",
    "&:hover": {
      textDecoration: "underline"
    }
  },
  issueLink: {
    textAlign: "center",
    fontSize: "10px",
    opacity: ".4",
    color: "inherit",
    textDecoration: "underline",
    transition: ".2s opacity",
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

  const {date} = useCardinalityState();
  const cardinalityDispatch = useCardinalityDispatch();

  const {search, pathname} = useLocation();
  const navigate = useNavigate();

  const [activeMenu, setActiveMenu] = useState(pathname);

  const headerSetup = useMemo(() => {
    return ((routerOptions[pathname] || {}) as RouterOptions).header || {};
  }, [pathname]);

  const onClickLogo = () => {
    navigateHandler(router.home);
    setQueryStringWithoutPageReload("");
    window.location.reload();
  };

  const navigateHandler = (pathname: string) => {
    navigate({pathname, search: search});
  };

  useEffect(() => {
    setActiveMenu(pathname);
  }, [pathname]);

  return <AppBar position="static" sx={{px: 1, boxShadow: "none"}}>
    <Toolbar>
      <Box display="grid" alignItems="center" justifyContent="center">
        <Box onClick={onClickLogo} sx={classes.logo}>
          <Logo style={{color: "inherit", marginRight: "6px"}}/>
          <Typography variant="h5">
            <span style={{fontWeight: "bolder"}}>VM</span>
            <span style={{fontWeight: "lighter"}}>UI</span>
          </Typography>
        </Box>
        <Link sx={classes.issueLink} target="_blank"
          href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new">
          create an issue
        </Link>
      </Box>
      <Box sx={{ml: 8}}>
        <Tabs value={activeMenu} textColor="inherit" TabIndicatorProps={{style: {background: "white"}}}
          onChange={(e, val) => setActiveMenu(val)}>
          <Tab label="Custom panel" value={router.home} component={RouterLink} to={`${router.home}${search}`}/>
          <Tab label="Dashboards" value={router.dashboards} component={RouterLink} to={`${router.dashboards}${search}`}/>
          <Tab
            label="Cardinality"
            value={router.cardinality}
            component={RouterLink}
            to={`${router.cardinality}${search}`}/>
        </Tabs>
      </Box>
      <Box display="grid" gridTemplateColumns="repeat(3, auto)" gap={1} alignItems="center" ml="auto" mr={0}>
        {headerSetup?.timeSelector && <TimeSelector/>}
        {headerSetup?.datePicker && (
          <DatePicker
            date={date}
            onChange={(val) => cardinalityDispatch({type: "SET_DATE", payload: val})}
          />
        )}
        {headerSetup?.executionControls && <ExecutionControls/>}
        {headerSetup?.globalSettings && <GlobalSettings/>}
      </Box>
    </Toolbar>
  </AppBar>;
};

export default Header;
