import SettingsIcon from "@mui/icons-material/Settings";
import React, {FC, useState} from "preact/compat";
import AxesLimitsConfigurator from "./AxesLimitsConfigurator";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Popover from "@mui/material/Popover";
import Typography from "@mui/material/Typography";
import makeStyles from "@mui/styles/makeStyles";
import CloseIcon from "@mui/icons-material/Close";

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

const GraphSettings: FC = () => {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

  const classes = useStyles();

  return <Box display="flex" px={2}>
    <Button variant="outlined" aria-describedby="settings-popover"
      onClick={(e) => setAnchorEl(e.currentTarget)} >
      <SettingsIcon sx={{fontSize: 16, marginRight: "4px"}}/>
      <span style={{lineHeight: 1, paddingTop: "1px"}}>{open ? "Hide" : "Show"} graph settings</span>
    </Button>
    <Popover
      id="settings-popover"
      open={open}
      anchorEl={anchorEl}
      onClose={() => setAnchorEl(null)}
      anchorOrigin={{
        vertical: "top",
        horizontal: anchorEl ? anchorEl.offsetWidth + 15 : 200
      }}
    >
      <Paper elevation={3} className={classes.popover}>
        <div id="handle" className={classes.popoverHeader}>
          <Typography variant="body1"><b>Graph Settings</b></Typography>
          <IconButton size="small" onClick={() => setAnchorEl(null)}>
            <CloseIcon style={{color: "white"}}/>
          </IconButton>
        </div>
        <Box className={classes.popoverBody}>
          <AxesLimitsConfigurator/>
        </Box>
      </Paper>
    </Popover>
  </Box>;
};

export default GraphSettings;