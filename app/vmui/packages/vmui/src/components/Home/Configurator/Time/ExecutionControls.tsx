import React, {FC, useEffect, useState} from "preact/compat";
import Tooltip from "@mui/material/Tooltip";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import Button from "@mui/material/Button";
import Popper from "@mui/material/Popper";
import Paper from "@mui/material/Paper";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import AutorenewIcon from "@mui/icons-material/Autorenew";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";

interface AutoRefreshOption {
  seconds: number
  title: string
}

const delayOptions: AutoRefreshOption[] = [
  {seconds: 0, title: "Off"},
  {seconds: 1, title: "1s"},
  {seconds: 2, title: "2s"},
  {seconds: 5, title: "5s"},
  {seconds: 10, title: "10s"},
  {seconds: 30, title: "30s"},
  {seconds: 60, title: "1m"},
  {seconds: 300, title: "5m"},
  {seconds: 900, title: "15m"},
  {seconds: 1800, title: "30m"},
  {seconds: 3600, title: "1h"},
  {seconds: 7200, title: "2h"}
];

export const ExecutionControls: FC = () => {

  const dispatch = useAppDispatch();
  const {queryControls: {autoRefresh}} = useAppState();

  const [selectedDelay, setSelectedDelay] = useState<AutoRefreshOption>(delayOptions[0]);

  const handleChange = (d: AutoRefreshOption) => {
    if ((autoRefresh && !d.seconds) || (!autoRefresh && d.seconds)) {
      dispatch({type: "TOGGLE_AUTOREFRESH"});
    }
    setSelectedDelay(d);
    setAnchorEl(null);
  };

  useEffect(() => {
    const delay = selectedDelay.seconds;
    let timer: number;
    if (autoRefresh) {
      timer = setInterval(() => {
        dispatch({type: "RUN_QUERY_TO_NOW"});
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
    <Tooltip title="Auto-refresh control">
      <Button variant="contained" color="primary"
        sx={{
          minWidth: "110px",
          color: "white",
          border: "1px solid rgba(0, 0, 0, 0.2)",
          justifyContent: "space-between",
          boxShadow: "none",
        }}
        startIcon={<AutorenewIcon/>}
        endIcon={<KeyboardArrowDownIcon sx={{transform: open ? "rotate(180deg)" : "none"}}/>}
        onClick={(e) => setAnchorEl(e.currentTarget)}
      >
        {selectedDelay.title}
      </Button>
    </Tooltip>
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="bottom-end"
      modifiers={[{name: "offset", options: {offset: [0, 6]}}]}>
      <ClickAwayListener onClickAway={() => setAnchorEl(null)}>
        <Paper elevation={3}>
          <List style={{minWidth: "110px",maxHeight: "208px", overflow: "auto", padding: "20px 0"}}>
            {delayOptions.map(d =>
              <ListItem key={d.seconds} button onClick={() => handleChange(d)}>
                <ListItemText primary={d.title}/>
              </ListItem>)}
          </List>
        </Paper>
      </ClickAwayListener></Popper>
  </>;
};