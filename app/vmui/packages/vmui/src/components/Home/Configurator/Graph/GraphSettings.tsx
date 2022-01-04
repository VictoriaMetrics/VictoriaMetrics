import SettingsIcon from "@mui/icons-material/Settings";
import React, {FC, useState} from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Popper from "@mui/material/Popper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import makeStyles from "@mui/styles/makeStyles";
import CloseIcon from "@mui/icons-material/Close";
import ClickAwayListener from "@mui/material/ClickAwayListener";

const useStyles = makeStyles({
  popover: {
    display: "grid",
    gridGap: "16px",
    padding: "0 0 25px",
  },
  popoverHeader: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    background: "#3F51B5",
    padding: "6px 6px 6px 12px",
    borderRadius: "4px 4px 0 0",
    color: "#FFF",
  },
  popoverBody: {
    display: "grid",
    gridGap: "6px",
    padding: "0 14px",
  }
});

const title = "Axes Settings";

const GraphSettings: FC = () => {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

  const classes = useStyles();

  return <Box>
    <Tooltip title={title}>
      <IconButton onClick={(e) => setAnchorEl(e.currentTarget)}>
        <SettingsIcon/>
      </IconButton>
    </Tooltip>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="left-start"
      modifiers={[{name: "offset", options: {offset: [0, 6]}}]}>
      <ClickAwayListener onClickAway={() => setAnchorEl(null)}>
        <Paper elevation={3} className={classes.popover}>
          <div id="handle" className={classes.popoverHeader}>
            <Typography variant="body1"><b>{title}</b></Typography>
            <IconButton size="small" onClick={() => setAnchorEl(null)}>
              <CloseIcon style={{color: "white"}}/>
            </IconButton>
          </div>
          <Box className={classes.popoverBody}>
            <AxesLimitsConfigurator/>
          </Box>
        </Paper>
      </ClickAwayListener>
    </Popper>
  </Box>;
};

export default GraphSettings;