import SettingsIcon from "@mui/icons-material/Settings";
import React, {FC, useState} from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Popper from "@mui/material/Popper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import {AxisRange, YaxisState} from "../../../../state/graph/reducer";

const classes = {
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
};

const title = "Axes Settings";

interface GraphSettingsProps {
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void
}

const GraphSettings: FC<GraphSettingsProps> = ({yaxis, setYaxisLimits, toggleEnableLimits}) => {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

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
        <Paper elevation={3} sx={classes.popover}>
          <Box id="handle" sx={classes.popoverHeader}>
            <Typography variant="body1"><b>{title}</b></Typography>
            <IconButton size="small" onClick={() => setAnchorEl(null)}>
              <CloseIcon style={{color: "white"}}/>
            </IconButton>
          </Box>
          <Box sx={classes.popoverBody}>
            <AxesLimitsConfigurator
              yaxis={yaxis}
              setYaxisLimits={setYaxisLimits}
              toggleEnableLimits={toggleEnableLimits}
            />
          </Box>
        </Paper>
      </ClickAwayListener>
    </Popper>
  </Box>;
};

export default GraphSettings;
