import React, { FC, useEffect, useState } from "preact/compat";
import Tooltip from "@mui/material/Tooltip";
import Button from "@mui/material/Button";
import Popper from "@mui/material/Popper";
import Paper from "@mui/material/Paper";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import AutorenewIcon from "@mui/icons-material/Autorenew";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import List from "@mui/material/List";
import ListItemText from "@mui/material/ListItemText";
import ListItemButton from "@mui/material/ListItemButton";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import {getAppModeEnable} from "../../../utils/app-mode";
import Box from "@mui/material/Box";

interface AutoRefreshOption {
  seconds: number
  title: string
}

const delayOptions: AutoRefreshOption[] = [
  { seconds: 0, title: "Off" },
  { seconds: 1, title: "1s" },
  { seconds: 2, title: "2s" },
  { seconds: 5, title: "5s" },
  { seconds: 10, title: "10s" },
  { seconds: 30, title: "30s" },
  { seconds: 60, title: "1m" },
  { seconds: 300, title: "5m" },
  { seconds: 900, title: "15m" },
  { seconds: 1800, title: "30m" },
  { seconds: 3600, title: "1h" },
  { seconds: 7200, title: "2h" }
];

export const ExecutionControls: FC = () => {

  const dispatch = useTimeDispatch();
  const appModeEnable = getAppModeEnable();
  const [autoRefresh, setAutoRefresh] = useState(false);

  const [selectedDelay, setSelectedDelay] = useState<AutoRefreshOption>(delayOptions[0]);

  const handleChange = (d: AutoRefreshOption) => {
    if ((autoRefresh && !d.seconds) || (!autoRefresh && d.seconds)) {
      setAutoRefresh(prev => !prev);
    }
    setSelectedDelay(d);
    setAnchorEl(null);
  };

  const handleUpdate = () => {
    dispatch({type: "RUN_QUERY"});
  };

  useEffect(() => {
    const delay = selectedDelay.seconds;
    let timer: number;
    if (autoRefresh) {
      timer = setInterval(() => {
        dispatch({ type: "RUN_QUERY" });
      }, delay * 1000) as unknown as number;
    } else {
      setSelectedDelay(delayOptions[0]);
    }
    return () => {
      timer && clearInterval(timer);
    };
  }, [selectedDelay, autoRefresh]);

  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
  const open = Boolean(anchorEl);

  return <>
    <Box sx={{
      minWidth: "110px",
      color: "white",
      border: appModeEnable ? "none" : "1px solid rgba(0, 0, 0, 0.2)",
      justifyContent: "space-between",
      boxShadow: "none",
      borderRadius: "4px",
      display: "grid",
      gridTemplateColumns: "auto 1fr"
    }}>
      <Tooltip title="Refresh dashboard">
        <Button variant="contained" color="primary"
          sx={{color: "white", minWidth: "34px", boxShadow: "none", borderRadius: "3px 0 0 3px", p: "6px 6px"}}
          startIcon={<AutorenewIcon fontSize={"small"} style={{marginRight: "-8px", marginLeft: "4px"}}/>}
          onClick={handleUpdate}
        >
        </Button>
      </Tooltip>
      <Tooltip title="Auto-refresh control">
        <Button variant="contained" color="primary" sx={{boxShadow: "none", borderRadius: "0 3px 3px 0"}} fullWidth
        endIcon={<KeyboardArrowDownIcon sx={{ transform: open ? "rotate(180deg)" : "none" }}/>}
        onClick={(e) => setAnchorEl(e.currentTarget)}
      >
        {selectedDelay.title}
      </Button>
    </Tooltip></Box>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="bottom-end"
      modifiers={[{ name: "offset", options: { offset: [0, 6] } }]}
    >
      <ClickAwayListener onClickAway={() => setAnchorEl(null)}>
        <Paper elevation={3}>
          <List style={{ minWidth: "110px", maxHeight: "208px", overflow: "auto", padding: "20px 0" }}>
            {delayOptions.map(d =>
              <ListItemButton
                key={d.seconds}
                onClick={() => handleChange(d)}
              >
                <ListItemText primary={d.title}/>
              </ListItemButton>)}
          </List>
        </Paper>
      </ClickAwayListener></Popper>
  </>;
};
