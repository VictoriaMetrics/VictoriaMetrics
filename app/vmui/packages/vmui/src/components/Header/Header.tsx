import React, {FC} from "react";
import {AppBar, Box, Link, Toolbar, Typography} from "@mui/material";
import {ExecutionControls} from "../Home/Configurator/Time/ExecutionControls";
import {DisplayTypeSwitch} from "../Home/Configurator/DisplayTypeSwitch";
import Logo from "../common/Logo";
import makeStyles from "@mui/styles/makeStyles";
import {setQueryStringWithoutPageReload} from "../../utils/query-string";

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
    position: "absolute",
    bottom: "6px",
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

  return <AppBar position="static">
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
      <Box ml={4} flexGrow={1}>
        <ExecutionControls/>
      </Box>
      <DisplayTypeSwitch/>
    </Toolbar>
  </AppBar>;
};

export default Header;