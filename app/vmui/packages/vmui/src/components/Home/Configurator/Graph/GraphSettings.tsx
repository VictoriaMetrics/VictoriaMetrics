import SettingsIcon from "@mui/icons-material/Settings";
import React, {FC, useState, useRef} from "react";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator";
import {Box, Button, IconButton, Paper, Typography} from "@mui/material";
import Draggable from "react-draggable";
import makeStyles from "@mui/styles/makeStyles";
import CloseIcon from "@mui/icons-material/Close";

const useStyles = makeStyles({
  popover: {
    position: "absolute",
    display: "grid",
    gridGap: "16px",
    padding: "0 0 25px",
    zIndex: 2,
  },
  popoverHeader: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    background: "#3F51B5",
    padding: "6px 6px 6px 12px",
    borderRadius: "4px 4px 0 0",
    color: "#FFF",
    cursor: "move",
  },
  popoverBody: {
    display: "grid",
    gridGap: "6px",
    padding: "0 14px",
  }
});

const GraphSettings: FC = () => {
  const [open, setOpen] = useState(false);
  const draggableRef = useRef<HTMLDivElement>(null);
  const position = { x: 173, y: 0 };

  const classes = useStyles();

  return <Box display="flex" px={2}>
    <Button onClick={() => setOpen((old) => !old)} variant="outlined">
      <SettingsIcon sx={{fontSize: 16, marginRight: "4px"}}/>
      <span style={{lineHeight: 1, paddingTop: "1px"}}>{open ? "Hide" : "Show"} graph settings</span>
    </Button>
    {open && (
      <Draggable nodeRef={draggableRef} defaultPosition={position} handle="#handle">
        <Paper elevation={3} className={classes.popover} ref={draggableRef}>
          <div id="handle" className={classes.popoverHeader}>
            <Typography variant="body1"><b>Graph Settings</b></Typography>
            <IconButton size="small" onClick={() => setOpen(false)}>
              <CloseIcon style={{color: "white"}}/>
            </IconButton>
          </div>
          <Box className={classes.popoverBody}>
            <AxesLimitsConfigurator/>
          </Box>
        </Paper>
      </Draggable>
    )}
  </Box>;
};

export default GraphSettings;