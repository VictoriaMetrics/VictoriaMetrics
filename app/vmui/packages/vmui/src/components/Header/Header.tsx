import React, {FC} from "preact/compat";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Link from "@mui/material/Link";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import {ExecutionControls} from "../Home/Configurator/Time/ExecutionControls";
import Logo from "../common/Logo";
import makeStyles from "@mui/styles/makeStyles";
import {setQueryStringWithoutPageReload} from "../../utils/query-string";
import {TimeSelector} from "../Home/Configurator/Time/TimeSelector";
import GlobalSettings from "../Home/Configurator/Settings/GlobalSettings";

const useStyles = makeStyles({
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
  }
});

const Header: FC = () => {

  const classes = useStyles();

  const onClickLogo = () => {
    setQueryStringWithoutPageReload("");
    window.location.reload();
  };

  return <AppBar position="static" sx={{px: 1, boxShadow: "none"}}>
    <Toolbar>
      <Box display="grid" alignItems="center" justifyContent="center">
        <Box onClick={onClickLogo} className={classes.logo}>
          <Logo style={{color: "inherit", marginRight: "6px"}}/>
          <Typography variant="h5">
            <span style={{fontWeight: "bolder"}}>VM</span>
            <span style={{fontWeight: "lighter"}}>UI</span>
          </Typography>
        </Box>
        <Link className={classes.issueLink} target="_blank"
          href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/new">
          create an issue
        </Link>
      </Box>
      <Box display="grid" gridTemplateColumns="repeat(3, auto)" gap={1} alignItems="center" ml="auto" mr={0}>
        <TimeSelector/>
        <ExecutionControls/>
        <GlobalSettings/>
      </Box>
    </Toolbar>
  </AppBar>;
};

export default Header;