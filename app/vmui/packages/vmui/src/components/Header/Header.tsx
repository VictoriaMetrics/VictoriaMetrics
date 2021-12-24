import React, {FC} from "preact/compat";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Link from "@mui/material/Link";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import {ExecutionControls} from "../Home/Configurator/Time/ExecutionControls";
import {DisplayTypeSwitch} from "../Home/Configurator/DisplayTypeSwitch";
import Logo from "../common/Logo";
import makeStyles from "@mui/styles/makeStyles";

const useStyles = makeStyles({
  logo: {
    position: "relative",
    display: "flex",
    alignItems: "center",
    color: "#fff",
    transition: ".2s textDecoration",
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

  return <AppBar position="static">
    <Toolbar>
      <Box display="grid" alignItems="center" justifyContent="center">
        <Link href="/" className={classes.logo}>
          <Logo style={{color: "inherit", marginRight: "6px"}}/>
          <Typography variant="h5">
            <span style={{fontWeight: "bolder"}}>VM</span>
            <span style={{fontWeight: "lighter"}}>UI</span>
          </Typography>
        </Link>
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